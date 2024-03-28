package source

import (
	"fmt"
	"log/slog"
	"os"

	"ruff.io/tio/rule/connector"
)

type Msg struct {
	ThingId string `json:"thingId"`
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
}

type MsgHander func(msg Msg)

type Source interface {
	Type() string
	Name() string
	// OnMsg a new handler is accepted with each invoke
	OnMsg(h MsgHander)
}

type Config struct {
	Name      string
	Type      string
	Connector string
	Options   map[string]any
}

type CreateFunc func(name string, cfg map[string]any, conn connector.Conn) Source

var registry map[string]CreateFunc = make(map[string]CreateFunc)

func Register(typ string, f CreateFunc) {
	if _, ok := registry[typ]; ok {
		slog.Error("Duplicate register sink", "type", typ)
		os.Exit(1)
	}
	registry[typ] = f
	slog.Info("Rule sink registered", "type", typ)
}

func New(cfg Config, conn connector.Conn) (Source, error) {
	f, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("source not found")
	}
	return f(cfg.Name, cfg.Options, conn), nil
}
