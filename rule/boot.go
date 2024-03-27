package rule

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/sink"
	"ruff.io/tio/rule/source"
)

var (
	conns   = make(map[string]connector.Conn)
	sinks   = make(map[string]sink.Sink)
	sources = make(map[string]source.Source)
	rules   = make(map[string]Rule)
)

// Read rule config, assemble rules and then boot them
//
// If config file is not exist, give up
func Boot(ctx context.Context) {
	cfg, err := ReadConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("Rule config file not found, if you do't need it, ignore this log", "error", err)
			return
		} else {
			slog.Error("Rule config file", "error", err)
			os.Exit(1)
		}
	}

	for _, cc := range cfg.Connectors {
		initConn(cc)
	}
	for _, sc := range cfg.Sources {
		initSource(sc)
	}
	for _, sc := range cfg.Sinks {
		initSink(sc)
	}

	// Crete rules
	for _, rc := range cfg.Rules {
		initRule(ctx, rc)
	}

	start()
}

func start() {
	for _, r := range rules {
		r.Start()
	}
}

func initRule(ctx context.Context, rc RuleConfig) {
	sks := make([]sink.Sink, 0)
	srcs := make([]source.Source, 0)
	for _, sn := range rc.Sinks {
		if sk, ok := sinks[sn]; ok {
			sks = append(sks, sk)
		} else {
			slog.Error("No sink for rule", "rule", rc.Name, "sink", sn)
			os.Exit(1)
		}
	}
	for _, sn := range rc.Sources {
		if src, ok := sources[sn]; ok {
			srcs = append(srcs, src)
		} else {
			slog.Error("No source for rule", "rule", rc.Name, "sink", sn)
			os.Exit(1)
		}
	}
	plist := make([]process.Process, 0)
	for _, cfg := range rc.Process {
		p, err := process.NewProcess(process.Config{
			Name: cfg.Name,
			Type: cfg.Type,
			Jq:   cfg.Jq,
		})
		if err != nil {
			slog.Error("Rule init process failed", "rule", rc.Name, "process", cfg.Name, "error", err)
			os.Exit(1)
		}
		plist = append(plist, p)
	}
	r := NewRule(ctx, rc.Name, srcs, plist, sks)
	if _, ok := rules[rc.Name]; ok {
		slog.Error("Rule name duplicated", "name", rc.Name)
		os.Exit(1)
	}
	rules[rc.Name] = r
}

func initConn(cfg connector.Config) {
	c, err := connector.New(cfg)
	if err != nil {
		slog.Error("Init rule connector", "name", cfg.Name, "type", cfg.Type, "error", err)
		os.Exit(1)
	}
	if _, ok := conns[cfg.Name]; ok {
		slog.Error("Duplicated name for rule connector", "name", cfg.Name)
		os.Exit(1)
	}
	conns[cfg.Name] = c
}

func initSink(cfg sink.Config) {
	c, ok := conns[cfg.Connector]
	if !ok {
		slog.Error("Init sink got no connector for amqp sink", "sinkName", cfg.Name, "connectorName", cfg.Connector)
		os.Exit(1)
	}
	s, err := sink.New(cfg, c)
	if err != nil {
		slog.Error("Init rule sink", "name", cfg.Name, "type", cfg.Type, "error", err)
		os.Exit(1)
	}
	if _, ok := sinks[cfg.Name]; ok {
		slog.Error("Duplicated name for rule sink", "name", cfg.Name)
		os.Exit(1)
	}
	sinks[cfg.Name] = s
}

func initSource(cfg source.Config) {
	var c connector.Conn
	if cfg.Connector != "" {
		cc, ok := conns[cfg.Connector]
		if !ok {
			slog.Error("Init sink got no connector for amqp sink", "sinkName", cfg.Name, "connectorName", cfg.Connector)
			os.Exit(1)
		} else {
			c = cc
		}
	}
	s, err := source.New(cfg, c)
	if err != nil {
		slog.Error("Init rule source", "name", cfg.Name, "type", cfg.Type, "error", err)
		os.Exit(1)
	}
	if _, ok := conns[cfg.Name]; ok {
		slog.Error("Duplicated name for rule source", "name", cfg.Name)
		os.Exit(1)
	}
	sources[cfg.Name] = s
}
