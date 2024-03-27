package sink

import (
	"fmt"
	"log/slog"
	"os"

	"ruff.io/tio/rule/connector"
)

type Msg struct {
	ThingId string
	Topic   string
	Payload []byte
}
type Sink interface {
	Type() string
	Name() string
	Publish(msg Msg)
}

type Config struct {
	Name      string
	Type      string
	Connector string
	Options   map[string]any
}

type CreateFunc func(name string, cfg map[string]any, conn connector.Conn) Sink

var registry map[string]CreateFunc = make(map[string]CreateFunc)

func Register(typ string, f CreateFunc) {
	if _, ok := registry[typ]; ok {
		slog.Error("Duplicate register sink", "type", typ)
		os.Exit(1)
	}
	registry[typ] = f
	slog.Info("Rule sink registered", "type", typ)
}

func New(cfg Config, conn connector.Conn) (Sink, error) {
	f, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("sink not found")
	}
	return f(cfg.Name, cfg.Options, conn), nil
}
