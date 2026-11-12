package integration_tests

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ruff.io/tio/auth"
	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	natsConn "ruff.io/tio/connector/nats"
	"ruff.io/tio/db/sqlite"
	mq "ruff.io/tio/internal/mqtttest"
	"ruff.io/tio/ntp"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
	shadowApi "ruff.io/tio/shadow/api"
	shadowWire "ruff.io/tio/shadow/wire"
	"ruff.io/tio/thing"
	thingApi "ruff.io/tio/thing/api"
	thingWire "ruff.io/tio/thing/wire"
)

var (
	cfg           = ReadConfig()
	dbConn        *gorm.DB
	natsConnector *natsConn.Connector
	shadowSvc     shadow.Service
	thingSvc      thing.Service
	httpSvr       *httptest.Server
	testCtx       context.Context
	testCancel    context.CancelFunc
	testCodec     codec.Codec
)

func TestMain(m *testing.M) {
	storeDir, err := os.MkdirTemp("", "tio-test-nats-*")
	if err != nil {
		log.Fatal(err)
	}
	cfg.Connector.Nats.Server.StoreDir = storeDir
	cfg.Connector.Nats.Server.Port = -1
	cfg.Connector.Nats.Server.MqttPort = -1
	cfg.Connector.Nats.Server.WsPort = -1
	cfg.Connector.Nats.Server.ClusterPort = 0
	if os.Getenv("TIO_TEST_MQTT_TLS") == "1" {
		certDir := filepath.Join("..", "demos", "mtls", "certs")
		cfg.Connector.Nats.Server.MqttTLS = config.NatsTLSConfig{
			CertFile: filepath.Join(certDir, "server-cert.pem"),
			KeyFile:  filepath.Join(certDir, "server-key.pem"),
			CAFile:   filepath.Join(certDir, "ca.pem"),
		}
	}

	if envEncoding := os.Getenv("TIO_TEST_ENCODING"); envEncoding != "" {
		cfg.Protocol.DefaultEncoding = envEncoding
	}
	if cfg.Protocol.DefaultEncoding == "" {
		cfg.Protocol.DefaultEncoding = "json"
	}

	if envMode := os.Getenv("TIO_TEST_PROTOCOL"); envMode != "" {
		cfg.Protocol.Mode = envMode
	}
	if cfg.Protocol.Mode == "" {
		cfg.Protocol.Mode = "legacy"
	}

	if err := cfg.Protocol.Validate(); err != nil {
		log.Fatalf("Invalid protocol config: %v", err)
	}

	dbConn, err = sqlite.Connect(sqlite.Config{FilePath: ":memory:"})
	if err != nil {
		log.Fatal(err)
	}
	autoMigrateDB(dbConn)

	testCtx, testCancel = context.WithCancel(context.Background())

	deviceCodec, err := codec.New(cfg.Protocol.DefaultEncoding)
	if err != nil {
		log.Fatal(err)
	}

	testCodec = deviceCodec
	natsConnector, err = natsConn.NewConnector(cfg.Connector.Nats, deviceCodec, cfg.Protocol.Mode)
	if err != nil {
		log.Fatal(err)
	}
	shadowSvc = shadowWire.InitSvc(dbConn, natsConnector, shadow.Config{})
	thingSvc = thingWire.InitSvc(testCtx, dbConn, shadowSvc, natsConnector)

	authzFn := auth.AuthzMqttClient(testCtx, cfg.Connector.Nats.SuperUsers, thingSvc, nil)

	if err := natsConnector.ConfigureAuth(authzFn, thingSvc); err != nil {
		log.Fatal(err)
	}
	if err := natsConnector.Start(testCtx); err != nil {
		log.Fatal(err)
	}

	natsConnector.OnLocalPresence(func(ci connector.ClientInfo) {
		go shadowSvc.HandleLocalPresence(ci)
	})

	shadowSvc.Init(testCtx)

	var methodHandler shadow.MethodHandler
	var simpleInvoker shadowApi.SimpleInvoker
	var simpleHandler *protocol.SimpleHandler
	profiles := thing.DeviceProfiles{Service: thingSvc, DefaultCodec: cfg.Protocol.DefaultEncoding, Mode: cfg.Protocol.Mode}
	legacyMethodHandler := shadow.NewMethodHandler(natsConnector, deviceCodec, profiles)
	shadowStateHandler := shadow.NewShadowHandler(natsConnector, deviceCodec, profiles)
	ntpHandler := ntp.NewNtpHandler(natsConnector, deviceCodec, profiles)

	if cfg.Protocol.Mode != "legacy" {
		simpleHandler, err = protocol.NewSimpleHandler(natsConnector, deviceCodec, shadowSvc, profiles)
		if err != nil {
			log.Fatalf("Create simple handler error: %v", err)
		}
		if err := simpleHandler.Start(testCtx); err != nil {
			log.Fatalf("Start simple handler error: %v", err)
		}
		log.Printf("Simple protocol handler started (defaultEncoding=%s)", cfg.Protocol.DefaultEncoding)
	}
	if cfg.Protocol.Mode != "simple" {
		if err := legacyMethodHandler.InitMethodHandler(testCtx); err != nil {
			log.Fatal(err)
		}
		if err := ntpHandler.InitNtpHandler(testCtx); err != nil {
			log.Fatal(err)
		}
		if err := shadow.Link(testCtx, shadowStateHandler, shadowSvc, shadow.Config{}); err != nil {
			log.Fatal(err)
		}
		log.Printf("Legacy protocol handlers started (defaultEncoding=%s)", cfg.Protocol.DefaultEncoding)
	}
	switch cfg.Protocol.Mode {
	case "legacy":
		methodHandler = legacyMethodHandler
	case "simple":
		simpleInvoker = simpleHandler
	case "dual":
		router := &protocol.MethodRouter{Profiles: profiles, Legacy: legacyMethodHandler, Simple: simpleHandler}
		methodHandler = router
		simpleInvoker = router
	}

	httpCon := restful.NewContainer()
	thingWs := thingApi.Service(testCtx, thingSvc)
	shadowApi.Service(testCtx, thingWs, shadowSvc, thingSvc, methodHandler)
	if simpleInvoker != nil {
		shadowApi.SimpleMethodService(testCtx, thingWs, simpleInvoker, thingSvc)
	}
	httpCon.Add(thingWs)

	httpSvr = httptest.NewServer(httpCon)

	time.Sleep(200 * time.Millisecond)

	code := m.Run()
	if simpleHandler != nil {
		simpleHandler.Stop()
	}
	httpSvr.Close()
	testCancel()
	if err := natsConnector.Shutdown(); err != nil {
		log.Printf("NATS connector shutdown: %v", err)
	}
	if err := os.RemoveAll(storeDir); err != nil {
		log.Printf("remove NATS store: %v", err)
	}
	os.Exit(code)
}

func autoMigrateDB(conn *gorm.DB) {
	err := conn.AutoMigrate(
		&thing.Entity{},
		&shadow.Entity{},
		&shadow.ConnStatusEntity{},
	)
	if err != nil {
		log.Fatal(err)
	}
}

func ID() string {
	id := uuid.NewV7()
	return "test-" + id.String()
}

func crateThing(thingId string) *thing.Thing {
	th, err := thingSvc.Create(testCtx, thing.Thing{
		Id:        thingId,
		Enabled:   true,
		AuthType:  "password",
		AuthValue: "test-password",
	}, nil, false)
	if err != nil {
		panic(fmt.Sprintf("create thing %q: %v", thingId, err))
	}
	return &th
}

func newThingMqttClient(_ context.Context, thingId, password string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	if cfg.Connector.Nats.Server.MqttTLS.CertFile != "" {
		client, err := mq.NewDeviceClientWithTLS(
			fmt.Sprintf("tls://127.0.0.1:%d", port),
			thingId,
			thingId,
			password,
			cfg.Connector.Nats.Server.MqttTLS.CAFile,
			"",
			"",
		)
		if err != nil {
			panic(err)
		}
		return client
	}
	return mq.NewDeviceClient(
		fmt.Sprintf("tcp://127.0.0.1:%d", port),
		thingId,
		thingId,
		password,
	)
}

func waitConnected(t *testing.T, thingId string) {
	t.Helper()
	require.Eventually(t, func() bool {
		connected, err := natsConnector.IsConnected(thingId)
		return err == nil && connected
	}, 5*time.Second, 50*time.Millisecond)
}

func waitDisconnected(t *testing.T, thingId string) {
	t.Helper()
	require.Eventually(t, func() bool {
		connected, err := natsConnector.IsConnected(thingId)
		return err == nil && !connected
	}, 5*time.Second, 50*time.Millisecond)
}
