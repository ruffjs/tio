package integration_tests

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ruff.io/tio/auth"
	"ruff.io/tio/connector"
	natsConn "ruff.io/tio/connector/nats"
	"ruff.io/tio/db/sqlite"
	mq "ruff.io/tio/internal/mqtttest"
	"ruff.io/tio/ntp"
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
)

func TestMain(m *testing.M) {
	storeDir, err := os.MkdirTemp("", "tio-test-nats-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(storeDir)

	cfg.Connector.Nats.Server.StoreDir = storeDir
	cfg.Connector.Nats.Server.Port = -1
	cfg.Connector.Nats.Server.MqttPort = -1
	cfg.Connector.Nats.Server.WsPort = -1
	cfg.Connector.Nats.Server.ClusterPort = 0

	dbConn, err = sqlite.Connect(sqlite.Config{FilePath: ":memory:"})
	if err != nil {
		log.Fatal(err)
	}
	autoMigrateDB(dbConn)

	natsConnector, err = natsConn.NewConnector(cfg.Connector.Nats)
	if err != nil {
		log.Fatal(err)
	}

	testCtx, testCancel = context.WithCancel(context.Background())
	defer testCancel()

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

	methodHandler := shadow.NewMethodHandler(natsConnector)
	shadowStateHandler := shadow.NewShadowHandler(natsConnector)
	ntpHandler := ntp.NewNtpHandler(natsConnector)

	if err := methodHandler.InitMethodHandler(testCtx); err != nil {
		log.Fatal(err)
	}
	if err := ntpHandler.InitNtpHandler(testCtx); err != nil {
		log.Fatal(err)
	}

	if err := shadow.Link(testCtx, shadowStateHandler, shadowSvc, shadow.Config{}); err != nil {
		log.Fatal(err)
	}

	httpCon := restful.NewContainer()
	thingWs := thingApi.Service(testCtx, thingSvc)
	shadowApi.Service(testCtx, thingWs, shadowSvc, thingSvc, methodHandler)
	httpCon.Add(thingWs)

	httpSvr = httptest.NewServer(httpCon)
	defer httpSvr.Close()

	time.Sleep(200 * time.Millisecond)

	os.Exit(m.Run())
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
	id, _ := uuid.NewV4()
	return "test-" + id.String()[:8]
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
