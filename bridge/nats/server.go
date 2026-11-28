package nats

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"ruff.io/tio/config"

	server "github.com/nats-io/nats-server/v2/server"
)

const (
	AppAccountName = "TIO_APP"
	SysAccountName = "TIO_SYS"
)

type NatsServer struct {
	server *server.Server
}

type brokerLogger struct{ *slog.Logger }

func (l brokerLogger) Warnf(format string, args ...any) {
	l.Warn(fmt.Sprintf(format, args...))
}

func (l brokerLogger) Errorf(format string, args ...any) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l brokerLogger) Fatalf(format string, args ...any) {
	// The embedding Tio process handles startup failures.
	l.Error(fmt.Sprintf(format, args...), "fatal", true)
}

func (brokerLogger) Noticef(string, ...any) {}
func (brokerLogger) Debugf(string, ...any)  {}
func (brokerLogger) Tracef(string, ...any)  {}

func StartNatsServer(cfg config.NatsServerConfig, js JetStreamOptions, authn server.Authentication) (*NatsServer, error) {
	opts, err := buildServerOptions(cfg, js, authn)
	if err != nil {
		return nil, fmt.Errorf("build server options: %w", err)
	}

	s, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("create nats server: %w", err)
	}

	s.SetLogger(brokerLogger{slog.With("component", "nats")}, false, false)
	s.Start()

	if !s.ReadyForConnections(10 * time.Second) {
		s.Shutdown()
		s.WaitForShutdown()
		return nil, fmt.Errorf("nats server not ready for connections within timeout")
	}

	if js.Enabled {
		appAcc, err := s.LookupAccount(AppAccountName)
		if err != nil {
			s.Shutdown()
			s.WaitForShutdown()
			return nil, fmt.Errorf("lookup APP account: %w", err)
		}
		if err := appAcc.EnableJetStream(nil, nil); err != nil {
			s.Shutdown()
			s.WaitForShutdown()
			return nil, fmt.Errorf("enable JetStream on APP account: %w", err)
		}
	}

	return &NatsServer{server: s}, nil
}

func (s *NatsServer) Shutdown() {
	s.server.Shutdown()
	s.server.WaitForShutdown()
}

func (s *NatsServer) IsRunning() bool {
	return s.server.Running()
}

func (s *NatsServer) ClientURL() string {
	return s.server.ClientURL()
}

func (s *NatsServer) Server() *server.Server {
	return s.server
}

func (s *NatsServer) logStartup() {
	v, err := s.server.Varz(nil)
	if err != nil {
		slog.Error("Read NATS startup information", "error", err)
		return
	}
	addr := func(host string, port int) string {
		if port <= 0 {
			return "disabled"
		}
		return net.JoinHostPort(host, strconv.Itoa(port))
	}
	monitor := "disabled"
	if a := s.server.MonitorAddr(); a != nil {
		monitor = a.String()
	}
	cluster := "disabled"
	if a := s.server.ClusterAddr(); a != nil {
		cluster = a.String()
	}
	slog.Info("NATS ready",
		"node", v.Name,
		"nats", addr(v.Host, v.Port), "natsTLS", v.TLSRequired,
		"monitor", monitor,
		"storeDir", s.server.StoreDir(),
		"clusterEnabled", s.server.ClusterAddr() != nil, "cluster", cluster)
}

func buildServerOptions(cfg config.NatsServerConfig, js JetStreamOptions, authn server.Authentication) (*server.Options, error) {
	opts := &server.Options{
		ServerName: cfg.ServerName,
		Port:       cfg.Port,
		HTTPPort:   cfg.MonitorPort,
		NoLog:      true,
		NoSigs:     true,
	}

	if js.Enabled {
		opts.JetStream = true
		opts.StoreDir = js.StoreDir
		opts.JetStreamMaxMemory = js.MaxMemory
		opts.JetStreamMaxStore = js.MaxStore
	}

	isClusterMode := cfg.ClusterPort != 0 || len(cfg.Routes) > 0

	if isClusterMode {
		opts.Cluster = server.ClusterOpts{
			Name:      cfg.ClusterName,
			Port:      cfg.ClusterPort,
			Advertise: cfg.ClusterAdvertise,
		}

		if cfg.ClusterUser != "" {
			opts.Cluster.Username = cfg.ClusterUser
			opts.Cluster.Password = cfg.ClusterPassword
		}

		if len(cfg.Routes) > 0 {
			routes, err := parseRoutes(cfg.Routes)
			if err != nil {
				return nil, fmt.Errorf("parse routes: %w", err)
			}
			opts.Routes = routes
		}

		if !tlsConfigEmpty(cfg.RouteTLS) {
			tlsCfg, err := buildTLSConfig(cfg.RouteTLS)
			if err != nil {
				return nil, fmt.Errorf("build route TLS config: %w", err)
			}
			opts.Cluster.TLSConfig = tlsCfg
		}
	}

	if !tlsConfigEmpty(cfg.ClientTLS) {
		tlsCfg, err := buildTLSConfig(cfg.ClientTLS)
		if err != nil {
			return nil, fmt.Errorf("build client TLS config: %w", err)
		}
		opts.TLSConfig = tlsCfg
		opts.TLS = true
	}

	appAccount := server.NewAccount(AppAccountName)
	sysAccount := server.NewAccount(SysAccountName)
	opts.Accounts = []*server.Account{appAccount, sysAccount}
	opts.SystemAccount = SysAccountName

	if authn != nil {
		opts.CustomClientAuthentication = authn
	}

	return opts, nil
}

func parseRoutes(routes []string) ([]*url.URL, error) {
	var urls []*url.URL
	for _, r := range routes {
		u, err := url.Parse(r)
		if err != nil {
			return nil, fmt.Errorf("parse route %q: %w", r, err)
		}
		urls = append(urls, u)
	}
	return urls, nil
}

func buildTLSConfig(cfg config.NatsTLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsCfg.ClientCAs = pool
		if cfg.RequireClientCert {
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		} else {
			tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
		}
	}

	return tlsCfg, nil
}

func tlsConfigEmpty(cfg config.NatsTLSConfig) bool {
	return cfg.CertFile == "" || cfg.KeyFile == ""
}
