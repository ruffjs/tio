package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ruff.io/tio/auth"
	"ruff.io/tio/metrics"
	"ruff.io/tio/ntp"
	"ruff.io/tio/rule"

	"ruff.io/tio"
	"ruff.io/tio/api"
	"ruff.io/tio/connector"
	natsConn "ruff.io/tio/connector/nats"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"gorm.io/gorm"
	"ruff.io/tio/config"
	"ruff.io/tio/db/mysql"
	"ruff.io/tio/db/sqlite"

	"ruff.io/tio/shadow"
	shadowWire "ruff.io/tio/shadow/wire"

	ruleApi "ruff.io/tio/rule/api"
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

func initLogger(cfg struct {
	Level string `json:"level,omitempty"`
}) {
	l, ok := map[string]slog.Level{
		"DEBUG": slog.LevelDebug,
		"INFO":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"ERROR": slog.LevelError,
	}[strings.ToUpper(cfg.Level)]
	if !ok {
		panic("Wrong log level config: " + cfg.Level)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
	slog.SetDefault(logger)
}

func main() {
	config.Version = Version
	config.GitCommit = GitCommit

	cfg := config.ReadConfig()

	initLogger(cfg.Log)

	slog.Info("Starting Tio", "version", Version, "gitCommit", GitCommit)
	slog.Info("Config",
		"apiPort", cfg.API.Port,
		"dbType", cfg.DB.Typ,
		"pprofEnabled", cfg.Pprof.Enabled)

	ctx, cancel := context.WithCancel(context.Background())
	config.GlobalCtxCancel = cancel
	go func() {
		if sig := signalHandler(ctx); sig != nil {
			cancel()
			slog.Info("Tio shutdown by signal", "signal", sig)
		}
	}()

	if cfg.Pprof.Enabled {
		go func() {
			api.StartPprofApiServer(ctx, cfg.Pprof.Port)
		}()
	}

	dbConn := newDb(cfg)
	autoMigrate(dbConn)

	natsConnector, err := natsConn.NewConnector(cfg.Connector.Nats)
	if err != nil {
		log.Fatalf("Create NATS connector error: %v", err)
	}

	var conn connector.Connector = natsConnector

	methodHandler := shadow.NewMethodHandler(conn)
	shadowStateHandler := shadow.NewShadowHandler(conn)
	ntpHandler := ntp.NewNtpHandler(conn)

	shadowSvc := shadowWire.InitSvc(dbConn, natsConnector, cfg.Shadow)
	thingSvc := thingWire.InitSvc(ctx, dbConn, shadowSvc, natsConnector)

	provisionSvc := thing.NewProvision(thingSvc, cfg.ProvisionSecret)
	if cfg.ProvisionSecret == "" {
		provisionSvc = nil
	}
	authzFn := auth.AuthzMqttClient(ctx, cfg.Connector.Nats.SuperUsers, thingSvc, provisionSvc)
	aclFn := auth.TopicAcl(thingSvc, cfg.Connector.Nats.SuperUsers)

	if err := natsConnector.ConfigureAuth(authzFn, aclFn); err != nil {
		log.Fatalf("Configure NATS auth error: %v", err)
	}
	if err := natsConnector.Start(ctx); err != nil {
		log.Fatalf("NATS connector start error: %v", err)
	}
	defer func() {
		if err := natsConnector.Shutdown(); err != nil {
			slog.Error("NATS connector shutdown error", "error", err)
		}
	}()

	natsConnector.OnLocalPresence(func(ci connector.ClientInfo) {
		go shadowSvc.HandleLocalPresence(ci)
	})

	shadowSvc.Init(ctx)

	ruleMgr := rule.NewRuleMgr()
	ruleMgr.Boot(ctx, shadowSvc)

	if err := methodHandler.InitMethodHandler(ctx); err != nil {
		log.Fatalf("Init method handler error: %v", err)
	}
	if err := ntpHandler.InitNtpHandler(ctx); err != nil {
		log.Fatalf("Init ntp handler error: %v", err)
	}

	if err := shadow.Link(ctx, shadowStateHandler, shadowSvc, cfg.Shadow); err != nil {
		log.Fatalf("Link shadow service to connector error %v", err)
	}

	httpCon := restful.NewContainer()

	tio.RouteSwagger(httpCon)
	tio.RouteWeb(httpCon)
	azf := api.BasicAuthMiddleware(cfg.API.BasicAuth.Name, cfg.API.BasicAuth.Password)
	thingWs := thingApi.Service(ctx, thingSvc).
		Filter(metrics.Middleware).
		Filter(api.LoggingMiddleware).
		Filter(azf)
	shadowApi.Service(ctx, thingWs, shadowSvc, thingSvc, methodHandler).
		Filter(metrics.Middleware).
		Filter(api.LoggingMiddleware).
		Filter(azf)
	cfgWs := config.Service(ctx, cfg).Filter(azf)

	ruleWs := ruleApi.Service(ctx, ruleMgr).
		Filter(api.LoggingMiddleware).Filter(azf)

	metricsWs := metrics.Service()

	httpCon.Add(thingWs)
	httpCon.Add(cfgWs)
	httpCon.Add(ruleWs)
	httpCon.Add(metricsWs)
	httpCon.Add(restfulspec.NewOpenAPIService(api.OpenapiConfig(httpCon)))
	if cfg.API.Cors {
		httpCon.Filter(restful.OPTIONSFilter())
	}
	startHttpSvr(ctx, cfg, httpCon)

	time.Sleep(1 * time.Second)
}

func startHttpSvr(ctx context.Context, cfg config.Config, container *restful.Container) {
	addr := fmt.Sprintf(":%d", cfg.API.Port)
	server := &http.Server{Addr: addr, Handler: container}
	errCh := make(chan error)
	go func() {
		slog.Info("Http listening", "addr", addr)

		slog.Info("Open http", "url", fmt.Sprintf("http://127.0.0.1%s", addr), "user", cfg.API.BasicAuth.Name, "password", cfg.API.BasicAuth.Password)
		errCh <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		ctxShutdown, cancelShutdown := context.WithTimeout(ctx, stopWaitTime)
		defer cancelShutdown()
		if err := server.Shutdown(ctxShutdown); err != nil {
			slog.Error("Http server error occurred during shutdown", "addr", addr, "error", err)
		}
		slog.Info("Http server shutdown", "addr", addr)
	case err := <-errCh:
		slog.Error("Http server exit cause", "error", err)
	}
}

func autoMigrate(conn *gorm.DB) {
	err := conn.AutoMigrate(
		&thing.Entity{},
		&shadow.Entity{},
		&shadow.ConnStatusEntity{},
	)
	if err != nil {
		log.Fatalf("auto migrate db error: %v", err)
	}
	time.Sleep(time.Millisecond * 100)
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
