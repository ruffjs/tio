package integration_tests

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	connector2 "ruff.io/tio/connector"

	"ruff.io/tio/connector/mqtt/embed"

	"ruff.io/tio/connector/mqtt/client"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"gorm.io/gorm"
	"ruff.io/tio/api"
	"ruff.io/tio/auth"
	"ruff.io/tio/config"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/db/mysql"
	"ruff.io/tio/db/sqlite"
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
	connector connector2.Connector
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
	slog.Info("Config", "config", cfgJ)
	ctx := context.Background()

	dbConn := newDb(cfg)
	autoMigrate(dbConn)

	// mqtt client
	mqttClient := client.NewClient(cfg.Connector.MqttClient)

	connector = mq.InitConnector(cfg.Connector, mqttClient)

	methodHandler := shadow.NewMethodHandler(connector)
	shadowStateHandler := shadow.NewShadowHandler(connector)

	shadowSvc = shadowWire.InitSvc(dbConn, connector, shadow.Config{})
	thingSvc = thingWire.InitSvc(ctx, dbConn, shadowSvc, connector)

	// embedded mqtt broker
	if cfg.Connector.Typ == config.ConnectorMqttEmbed {
		startMqttBroker(ctx, cfg.Connector.MqttBroker, thingSvc)
	}

	if err := mqttClient.Connect(ctx); err != nil {
		log.Fatalf("Mqtt client start error: %v", err)
	}
	if err := methodHandler.InitMethodHandler(ctx); err != nil {
		log.Fatalf("Init method handler error: %v", err)
	}
	if err := shadow.Link(ctx, shadowStateHandler, shadowSvc, shadow.Config{}); err != nil {
		log.Fatalf("Link shadow service to connector error %v", err)
	}

	container := restful.NewContainer()
	container.ServeMux = http.NewServeMux()
	thingWs := thingApi.Service(context.Background(), thingSvc)
	shadowApi.Service(context.Background(), thingWs, shadowSvc, thingSvc, methodHandler)
	container.Add(thingWs)
	container.Add(restfulspec.NewOpenAPIService(api.OpenapiConfig(container)))

	// http test server
	httpSvr = httptest.NewServer(container)

	slog.Info("================ set environment done ================")

}

func crateThing(id string) thing.Thing {
	th, err := thingSvc.Create(context.Background(), thing.Thing{Id: id, Enabled: true}, nil, false)
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
		log.Fatalf("Unknown database type: %v", cfg.DB.Typ)
	}
	return nil
}

func newSqliteDB(cfg sqlite.Config) *gorm.DB {
	db, err := sqlite.Connect(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	return db
}

func newMysqlDB(cfg mysql.Config) *gorm.DB {
	conn, err := mysql.Connect(cfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	return conn
}

func startMqttBroker(ctx context.Context, cfg config.InnerMqttBroker, thingSvc thing.Service) {
	embed.InitBroker(embed.MochiConfig{
		TcpPort:           cfg.TcpPort,
		TcpSslPort:        cfg.TcpSslPort,
		CertFile:          cfg.CertFile,
		KeyFile:           cfg.KeyFile,
		ClientCAFile:      cfg.ClientCAFile,
		RequireClientCert: cfg.RequireClientCert,
		AuthzFn:           auth.AuthzMqttClient(ctx, cfg.SuperUsers, thingSvc, nil),
		AclFn:             auth.TopicAcl(thingSvc, cfg.SuperUsers),
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

func newThingMTLSClient(thingId string) client.Client {
	certDir := filepath.Join("..", "demos", "mtls", "certs")
	return newThingMTLSClientWithTLS(config.MqttClientConfig{
		ClientId: thingId,
		Host:     "localhost",
		Port:     cfg.Connector.MqttBroker.TcpSslPort,
		TLS: &config.TLSConfig{
			CAFile:     filepath.Join(certDir, "ca.pem"),
			CertFile:   filepath.Join(certDir, "client-cert.pem"),
			KeyFile:    filepath.Join(certDir, "client-key.pem"),
			ServerName: "localhost",
		},
	})
}

func newThingMTLSClientWithTLS(cfg config.MqttClientConfig) client.Client {
	return client.NewClient(cfg)
}

func newThingMTLSClientWithCertFiles(thingId, certFile, keyFile, caFile, serverName string) client.Client {
	return newThingMTLSClientWithConfig(config.MqttClientConfig{
		ClientId: thingId,
		Host:     "localhost",
		Port:     cfg.Connector.MqttBroker.TcpSslPort,
		TLS: &config.TLSConfig{
			CAFile:     caFile,
			CertFile:   certFile,
			KeyFile:    keyFile,
			ServerName: serverName,
		},
	})
}

func newThingMTLSClientWithConfig(cfg config.MqttClientConfig) client.Client {
	return newThingMTLSClientWithTLS(cfg)
}

func newThingMTLSClientWithUsername(clientID, username, certFile, keyFile, caFile, serverName string) client.Client {
	return newThingMTLSClientWithTLS(config.MqttClientConfig{
		ClientId: clientID,
		User:     username,
		Host:     "localhost",
		Port:     cfg.Connector.MqttBroker.TcpSslPort,
		TLS: &config.TLSConfig{
			CAFile:     caFile,
			CertFile:   certFile,
			KeyFile:    keyFile,
			ServerName: serverName,
		},
	})
}

func newThingMTLSClientWithServerName(thingId, serverName string) client.Client {
	certDir := filepath.Join("..", "demos", "mtls", "certs")
	return newThingMTLSClientWithCertFiles(
		thingId,
		filepath.Join(certDir, "client-cert.pem"),
		filepath.Join(certDir, "client-key.pem"),
		filepath.Join(certDir, "ca.pem"),
		serverName,
	)
}

func newThingTLSClientWithoutCertificate(thingId string) client.Client {
	certDir := filepath.Join("..", "demos", "mtls", "certs")
	return newThingMTLSClientWithTLS(config.MqttClientConfig{
		ClientId: thingId,
		Host:     "localhost",
		Port:     cfg.Connector.MqttBroker.TcpSslPort,
		TLS: &config.TLSConfig{
			CAFile:     filepath.Join(certDir, "ca.pem"),
			ServerName: "localhost",
		},
	})
}

func waitConnected(t *testing.T, thingId string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := connector.IsConnected(thingId)
		if err == nil && ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	ok, err := connector.IsConnected(thingId)
	if err != nil {
		t.Fatalf("check thing %s connected failed: %v", thingId, err)
	}
	t.Fatalf("thing %s was not connected, final state=%v", thingId, ok)
}

func waitDisconnected(t *testing.T, thingId string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := connector.IsConnected(thingId)
		if err == nil && !ok {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	ok, err := connector.IsConnected(thingId)
	if err != nil {
		t.Fatalf("check thing %s connected failed: %v", thingId, err)
	}
	t.Fatalf("thing %s was still connected, final state=%v", thingId, ok)
}

var uuidProv = uuid.New()

func ID() string {
	id, _ := uuidProv.ID()
	return id
}
