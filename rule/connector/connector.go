package connector

import (
	"context"
	"fmt"
	"log/slog"

	"ruff.io/tio/rule/model"
)

type Conn interface {
	Name() string
	Type() string
	Start() error
	Stop() error
	Status() model.StatusInfo
}

type Config struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Options map[string]interface{} `json:"options"`
}

type CreateFunc func(ctx context.Context, name string, cfg map[string]any) (Conn, error)

var registry map[string]CreateFunc = make(map[string]CreateFunc)

func Register(typ string, f CreateFunc) {
	if _, ok := registry[typ]; ok {
		slog.Error("Duplicate register connector", "type", typ)
		return
	}
	registry[typ] = f
	slog.Info("Rule connector registered", "type", typ)
}

func New(ctx context.Context, cfg Config) (Conn, error) {
	f, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("connector not found")
	}
	return f(ctx, cfg.Name, cfg.Options)
}
