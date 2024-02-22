package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ruff.io/tio/job"
	"ruff.io/tio/ntp"

	"ruff.io/tio"
	"ruff.io/tio/api"
	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/connector/mqtt/embed"

	"ruff.io/tio/auth/password"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"gorm.io/gorm"
	"ruff.io/tio/config"
	mq "ruff.io/tio/connector/mqtt"
	"ruff.io/tio/db/mysql"
	"ruff.io/tio/db/sqlite"
	"ruff.io/tio/pkg/log"

	"ruff.io/tio/shadow"
	shadowWire "ruff.io/tio/shadow/wire"

	jobApi "ruff.io/tio/job/api"
	jobWire "ruff.io/tio/job/wire"
	shadowApi "ruff.io/tio/shadow/api"
	"ruff.io/tio/thing"
	thingApi "ruff.io/tio/thing/api"
	thingWire "ruff.io/tio/thing/wire"
)

var (
	Version   = ""
	GitCommit = ""
)

const (
	stopWaitTime = time.Second * 1
)

func main() {
	config.Version = Version
	config.GitCommit = GitCommit

	// load config
	cfg := config.ReadConfig()
	cfgJ, _ := json.Marshal(cfg)

	// init logger
	log.Init(cfg.Log)

	log.Infof("Version: %s GitCommit: %s", Version, GitCommit)
	log.Infof("Config: %s", cfgJ)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if sig := signalHandler(ctx); sig != nil {
			cancel()
			log.Info(fmt.Sprintf("Tio shutdown by signal: %s", sig))
		}
	}()

	dbConn := newDb(cfg)
	autoMigrate(dbConn)

	// mqtt client and connector for tio interacts with message broker

	mqttClient := client.NewClient(cfg.Connector.MqttClient)
	connector := mq.InitConnector(cfg.Connector, mqttClient)

	methodHandler := shadow.NewMethodHandler(connector)
	shadowStateHandler := shadow.NewShadowHandler(connector)
	ntpHandler := ntp.NewNtpHandler(connector)

	// services
	shadowSvc := shadowWire.InitSvc(dbConn, connector)
	thingSvc := thingWire.InitSvc(ctx, dbConn, shadowSvc, connector)

	jobCenter := job.NewCenter(job.CenterOptions{
		ScheduleInterval:       time.Millisecond * 100,
		CheckJobStatusInterval: time.Millisecond * 100,
	}, job.NewRepo(dbConn), connector, connector, methodHandler, shadowSvc)
	jobMgrSvc := jobWire.InitSvc(dbConn, jobCenter)

	// embedded mqtt broker
	if cfg.Connector.Typ == config.ConnectorMqttEmbed {
		authzFn := password.AuthzMqttClient(ctx, cfg.Connector.MqttBroker.SuperUsers, thingSvc)
		startMqttBroker(ctx, cfg.Connector.MqttBroker, authzFn)
	}

	// init
	if err := connector.Start(ctx); err != nil {
		log.Fatalf("Mqtt connector start error: %v", err)
	}
	if err := shadowSvc.SyncConnStatus(ctx); err != nil {
		log.Fatalf("Sync Conn Status error: %v", err)
	}
	if err := methodHandler.InitMethodHandler(ctx); err != nil {
		log.Fatalf("Init method handler error: %v", err)
	}
	if err := ntpHandler.InitNtpHandler(ctx); err != nil {
		log.Fatalf("Init ntp handler error: %v", err)
	}

	if err := shadow.Link(ctx, shadowStateHandler, shadowSvc); err != nil {
		log.Fatalf("Link shadow service to connector error %v", err)
	}
	if err := mqttClient.Connect(ctx); err != nil {
		log.Fatalf("Mqtt client start error: %v", err)
	}
	if err := jobCenter.Start(ctx); err != nil {
		log.Fatalf("JobCenter start error: %v", err)
	}

	// htt api

	tio.RouteSwagger()
	tio.RouteWeb()
	azf := api.BasicAuthMiddleware(cfg.API.BasicAuth.Name, cfg.API.BasicAuth.Password)
	thingWs := thingApi.Service(ctx, thingSvc).
		Filter(api.LoggingMiddleware).
		Filter(azf)
	shadowApi.Service(ctx, thingWs, shadowSvc, thingSvc, methodHandler)

	jobWs := jobApi.Service(ctx, jobMgrSvc, thingWs)
	jobWs.Filter(api.LoggingMiddleware).Filter(azf)

	mqWs := mq.Service(ctx, connector).Filter(api.LoggingMiddleware).Filter(azf)

	restful.DefaultContainer.Add(thingWs)
	restful.DefaultContainer.Add(mqWs)
	restful.DefaultContainer.Add(jobWs)
	restful.DefaultContainer.Add(thingApi.ServiceForEmqxIntegration())
	restful.DefaultContainer.Add(restfulspec.NewOpenAPIService(api.OpenapiConfig()))
	if cfg.API.Cors {
		restful.DefaultContainer.Filter(restful.OPTIONSFilter())
	}
	startHttpSvr(ctx, cfg, nil)

	// wait some seconds before shutting down
	time.Sleep(1 * time.Second)
}

func startHttpSvr(ctx context.Context, cfg config.Config, handler http.Handler) {
	addr := fmt.Sprintf(":%d", cfg.API.Port)
	server := &http.Server{Addr: addr, Handler: handler}
	errCh := make(chan error)
	go func() {
		log.Infof("Http listening on %s", addr)

		log.Infof("Open http://127.0.0.1%s user=%s password=%s",
			addr, cfg.API.BasicAuth.Name, cfg.API.BasicAuth.Password)
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		ctxShutdown, cancelShutdown := context.WithTimeout(ctx, stopWaitTime)
		defer cancelShutdown()
		if err := server.Shutdown(ctxShutdown); err != nil {
			log.Errorf("Http server error occurred during shutdown at %s: %s", addr, err)
		}
		log.Info(fmt.Sprintf("Http server shutdown of http at %s", addr))
	case err := <-errCh:
		log.Errorf("Http server exit cause: %v", err)
	}
}

func autoMigrate(conn *gorm.DB) {
	err := conn.AutoMigrate(
		&thing.Entity{},
		&shadow.Entity{},
		&shadow.ConnStatusEntity{},
		&job.Entity{},
		&job.TaskEntity{},
	)
	if err != nil {
		log.Fatalf("auto migrate db error: %v", err)
	}
	time.Sleep(time.Millisecond * 100)
}

func startMqttBroker(ctx context.Context, cfg config.InnerMqttBroker, authzFn embed.AuthzFn) embed.Broker {
	return embed.InitBroker(embed.MochiConfig{
		TcpPort:    cfg.TcpPort,
		TcpSslPort: cfg.TcpSslPort,
		WsPort:     cfg.WsPort,
		WssPort:    cfg.WssPort,
		KeyFile:    cfg.KeyFile,
		CertFile:   cfg.CertFile,
		Storage:    cfg.Storage,
		AuthzFn:    authzFn,
		AclFn: func(user string, topic string, write bool) bool {
			return thing.TopicAcl(cfg.SuperUsers, user, topic, write)
		},
		SuperUsers: cfg.SuperUsers,
	})
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

func signalHandler(ctx context.Context) error {
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGABRT)
	select {
	case sig := <-c:
		return fmt.Errorf("%s", sig)
	case <-ctx.Done():
		return nil
	}
}
