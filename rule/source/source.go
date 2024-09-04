package source

import (
	"context"
	"fmt"
	"log/slog"

	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
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
	Start() error
	Stop()
	Status() model.StatusInfo
	// OnMsg a new handler is accepted with each invoke
	OnMsg(ruleName string, h MsgHander)
}

type Config struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Connector string         `json:"connector"`
	Enabled   bool           `json:"enabled"`
	Options   map[string]any `json:"options"`
}

type CreateFunc func(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Source, error)

var registry map[string]CreateFunc = make(map[string]CreateFunc)

func Register(typ string, f CreateFunc) {
	if _, ok := registry[typ]; ok {
		slog.Error("Duplicate register sink", "type", typ)
		return
	}
	registry[typ] = f
	slog.Info("Rule sink registered", "type", typ)
}

func New(ctx context.Context, cfg Config, conn connector.Conn) (Source, error) {
	f, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("source not found")
	}
	return f(ctx, cfg.Name, cfg.Options, conn)
}
