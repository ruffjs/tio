package integration_tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ruff.io/tio/connector/mqtt/embed"

	"ruff.io/tio/connector/mqtt/client"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"gorm.io/gorm"
	"ruff.io/tio/api"
	"ruff.io/tio/auth/password"
	"ruff.io/tio/config"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/db/mysql"
	"ruff.io/tio/db/sqlite"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/uuid"
	"ruff.io/tio/shadow"
	shadowApi "ruff.io/tio/shadow/api"
	shadowWire "ruff.io/tio/shadow/wire"
	"ruff.io/tio/thing"
	thingApi "ruff.io/tio/thing/api"
	thingWire "ruff.io/tio/thing/wire"
)

var (
	cfg     config.Config
	httpSvr *httptest.Server

	thingSvc  thing.Service
	shadowSvc shadow.Service
	connector shadow.Connector
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	// httpSvr.Disconnect()
	// mqttSvr.Disconnect()
	os.Exit(code)
}

func setup() {
	cfg = ReadConfig()
	cfgJ, _ := json.Marshal(cfg)
	log.Infof("Config: %s", cfgJ)
	ctx := context.Background()

	dbConn := newDb(cfg)
	autoMigrate(dbConn)

	// mqtt client
	mqttClient := client.NewClient(cfg.Connector.MqttClient)

	connector = mq.InitConnector(cfg.Connector, mqttClient)

	shadowSvc = shadowWire.InitSvc(dbConn, connector)
	thingSvc = thingWire.InitSvc(ctx, dbConn, shadowSvc, connector)

	// embedded mqtt broker
	if cfg.Connector.Typ == config.ConnectorMqttEmbed {
		startMqttBroker(ctx, cfg.Connector.MqttBroker, thingSvc)
	}

	if err := mqttClient.Connect(ctx); err != nil {
		log.Fatalf("Mqtt client start error: %v", err)
	}
	if err := connector.InitMethodHandler(ctx); err != nil {
		log.Fatalf("Connector init method handler error: %v", err)
	}
	if err := shadow.Link(ctx, connector, shadowSvc); err != nil {
		log.Fatalf("Link shadow service to connector error %v", err)
	}

	container := restful.NewContainer()
	container.ServeMux = http.NewServeMux()
	thingWs := thingApi.Service(context.Background(), thingSvc)
	shadowApi.Service(context.Background(), thingWs, shadowSvc, thingSvc, connector)
	container.Add(thingWs)
	container.Add(restfulspec.NewOpenAPIService(api.OpenapiConfig()))

	// http test server
	httpSvr = httptest.NewServer(container)

	log.Info("================ set environment done ================")

}

func crateThing(id string) thing.Thing {
	th, err := thingSvc.Create(context.Background(), thing.Thing{Id: id})
	if err != nil {
		log.Fatalf("Create thing error %v", err)
	}
	return th
}

func newDb(cfg config.Config) *gorm.DB {
	switch cfg.DB.Typ {
	case config.DBMySQL:
		return newMysqlDB(cfg.DB.Mysql)
	case config.DBSqlite:
		return newSqliteDB(cfg.DB.Sqlite)
	default:
		log.Fatal("Unknown database type: ", cfg.DB.Typ)
	}
	return nil
}

func newSqliteDB(cfg sqlite.Config) *gorm.DB {
	db, err := sqlite.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func newMysqlDB(cfg mysql.Config) *gorm.DB {
	conn, err := mysql.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	return conn
}

func startMqttBroker(ctx context.Context, cfg config.InnerMqttBroker, thingSvc thing.Service) {
	embed.InitBroker(embed.MochiConfig{
		TcpPort: cfg.TcpPort,
		AuthzFn: password.AuthzMqttClient(ctx, cfg.SuperUsers, thingSvc),
		AclFn: func(user string, topic string, write bool) bool {
			return true
		},
	})
}

func autoMigrate(conn *gorm.DB) {
	_ = conn.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{})
}

func newThingMqttClient(cxt context.Context, thingId string, password string) client.Client {
	c := config.MqttClientConfig{
		ClientId: thingId,
		User:     thingId,
		Password: password,
		Host:     cfg.Connector.MqttClient.Host,
		Port:     cfg.Connector.MqttClient.Port,
	}
	return client.NewClient(c)
}

var uuidProv = uuid.New()

func ID() string {
	id, _ := uuidProv.ID()
	return id
}
