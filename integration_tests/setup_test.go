package integration_tests

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ruff.io/tio/api"
	"ruff.io/tio/auth"
	natsbridge "ruff.io/tio/bridge/nats"
	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/connector/mqtt/embed"
	mqtthttp "ruff.io/tio/connector/mqtt"
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
	cfg         = ReadConfig()
	dbConn      *gorm.DB
	mqttBroker  *embed.Broker
	natsBrg     *natsbridge.Bridge
	shadowSvc   shadow.Service
	thingSvc    thing.Service
	httpSvr     *httptest.Server
	testCtx     context.Context
	testCancel  context.CancelFunc
	testCodec   codec.Codec
)

func TestMain(m *testing.M) {
	mqttStoreDir, err := os.MkdirTemp("", "tio-test-mqtt-*")
	if err != nil {
		log.Fatal(err)
	}
	natsStoreDir, err := os.MkdirTemp("", "tio-test-nats-*")
	if err != nil {
		log.Fatal(err)
	}

	cfg.Connector.MqttBroker.TcpPort = freePort()
	cfg.Connector.MqttBroker.WsPort = freePort()
	cfg.Connector.MqttBroker.Storage = config.InnerMqttStorage{Type: "file", FilePath: filepath.Join(mqttStoreDir, "comqtt.db")}

	cfg.Nats.Enabled = true
	cfg.Nats.Mode = "embedded"
	cfg.Nats.Server.Port = -1
	cfg.Nats.Server.MonitorPort = 0
	cfg.Nats.JetStream.Enabled = false
	cfg.Nats.JetStream.StoreDir = natsStoreDir

	if os.Getenv("TIO_TEST_MQTT_TLS") == "1" {
		certDir := filepath.Join("..", "demos", "mtls", "certs")
		cfg.Connector.MqttBroker.TcpSslPort = freePort()
		cfg.Connector.MqttBroker.CertFile = filepath.Join(certDir, "server-cert.pem")
		cfg.Connector.MqttBroker.KeyFile = filepath.Join(certDir, "server-key.pem")
		cfg.Connector.MqttBroker.ClientCAFile = filepath.Join(certDir, "ca.pem")
		cfg.Connector.MqttBroker.RequireClientCert = true
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

	natsBrg, err = natsbridge.New(natsbridge.FromConfig(cfg.Nats), cfg.Connector.MqttBroker.SuperUsers)
	if err != nil {
		log.Fatal(err)
	}
	if err := natsBrg.Start(testCtx); err != nil {
		log.Fatal(err)
	}

	mqttBroker = embed.InitBroker(embed.Options{
		TcpPort:           cfg.Connector.MqttBroker.TcpPort,
		TcpSslPort:        cfg.Connector.MqttBroker.TcpSslPort,
		WsPort:            cfg.Connector.MqttBroker.WsPort,
		WssPort:           cfg.Connector.MqttBroker.WssPort,
		CertFile:          cfg.Connector.MqttBroker.CertFile,
		KeyFile:           cfg.Connector.MqttBroker.KeyFile,
		ClientCAFile:      cfg.Connector.MqttBroker.ClientCAFile,
		RequireClientCert: cfg.Connector.MqttBroker.RequireClientCert,
		Storage:           cfg.Connector.MqttBroker.Storage,
		Codec:             deviceCodec,
		ProtocolMode:      cfg.Protocol.Mode,
		Bridge:            natsBrg,
	})

	shadowSvc = shadowWire.InitSvc(dbConn, mqttBroker, shadow.Config{})
	thingSvc = thingWire.InitSvc(testCtx, dbConn, shadowSvc, mqttBroker)

	authzFn := auth.AuthzMqttClient(testCtx, cfg.Connector.MqttBroker.SuperUsers, thingSvc, nil,
		cfg.Auth.AllowPasswordBootstrap)
	aclFn := auth.TopicAcl(thingSvc, cfg.Connector.MqttBroker.SuperUsers, cfg.Protocol.Mode)
	if err := mqttBroker.ConfigureAuth(authzFn, aclFn); err != nil {
		log.Fatal(err)
	}

	mqttBroker.OnLocalPresence(func(ci connector.ClientInfo) {
		go shadowSvc.HandleLocalPresence(testCtx, ci)
	})
	if err := mqttBroker.Start(testCtx); err != nil {
		log.Fatal(err)
	}

	shadowSvc.Init(testCtx)

	var methodHandler shadow.MethodHandler
	var simpleHandler *protocol.SimpleHandler
	profiles := thing.DeviceProfiles{Service: thingSvc, DefaultCodec: cfg.Protocol.DefaultEncoding, Mode: cfg.Protocol.Mode}
	legacyMethodHandler := shadow.NewMethodHandler(mqttBroker, deviceCodec, profiles)
	shadowStateHandler := shadow.NewShadowHandler(mqttBroker, deviceCodec, profiles)
	ntpHandler := ntp.NewNtpHandler(mqttBroker, deviceCodec, profiles)

	if cfg.Protocol.Mode != "legacy" {
		simpleHandler, err = protocol.NewSimpleHandler(mqttBroker, deviceCodec, shadowSvc, profiles)
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
	case "simple", "dual":
		methodHandler = &protocol.MethodRouter{Profiles: profiles, Legacy: legacyMethodHandler, Simple: simpleHandler}
	}

	httpCon := restful.NewContainer()
	thingWs := thingApi.Service(testCtx, thingSvc)
	shadowApi.Service(testCtx, thingWs, shadowSvc, thingSvc, methodHandler)
	httpCon.Add(thingWs)
	httpCon.Add(api.MQTTBrokerService(mqttBroker.CloseAll, thingSvc, nil).
		Filter(api.BasicAuthMiddleware(cfg.API.BasicAuth.Name, cfg.API.BasicAuth.Password)))
	httpCon.Add(mqtthttp.Service(testCtx, mqttBroker))

	httpSvr = httptest.NewServer(httpCon)

	time.Sleep(200 * time.Millisecond)

	code := m.Run()
	if simpleHandler != nil {
		simpleHandler.Stop()
	}
	httpSvr.Close()
	testCancel()
	if err := natsBrg.Shutdown(); err != nil {
		log.Printf("NATS bridge shutdown: %v", err)
	}
	if err := os.RemoveAll(natsStoreDir); err != nil {
		log.Printf("remove NATS store: %v", err)
	}
	if err := os.RemoveAll(mqttStoreDir); err != nil {
		log.Printf("remove MQTT store: %v", err)
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

func freePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		log.Fatal(err)
	}
	return port
}

func mqttPort() int { return cfg.Connector.MqttBroker.TcpPort }
func wsPort() int   { return cfg.Connector.MqttBroker.WsPort }

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
	port := mqttPort()
	if cfg.Connector.MqttBroker.CertFile != "" && cfg.Connector.MqttBroker.RequireClientCert {
		client, err := mq.NewDeviceClientWithTLS(
			fmt.Sprintf("tls://127.0.0.1:%d", cfg.Connector.MqttBroker.TcpSslPort),
			thingId,
			thingId,
			password,
			cfg.Connector.MqttBroker.ClientCAFile,
			"",
			"",
		)
		if err != nil {
			panic(err)
		}
		return client
	}
	if cfg.Connector.MqttBroker.CertFile != "" && !cfg.Connector.MqttBroker.RequireClientCert {
		client, err := mq.NewDeviceClientWithTLS(
			fmt.Sprintf("tls://127.0.0.1:%d", cfg.Connector.MqttBroker.TcpSslPort),
			thingId,
			thingId,
			password,
			cfg.Connector.MqttBroker.ClientCAFile,
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
		connected, err := mqttBroker.IsConnected(thingId)
		return err == nil && connected
	}, 5*time.Second, 50*time.Millisecond)
}

func waitDisconnected(t *testing.T, thingId string) {
	t.Helper()
	require.Eventually(t, func() bool {
		connected, err := mqttBroker.IsConnected(thingId)
		return err == nil && !connected
	}, 5*time.Second, 50*time.Millisecond)
}
