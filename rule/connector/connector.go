package connector

import (
	"fmt"
	"log/slog"
	"os"
)

type Status string

const (
	StatusConnected    Status = "connected"
	StatusDisconnected Status = "disconnected"
)

type Conn interface {
	Name() string
	Type() string
	Connect() error
	Close() error
	Status() Status
}

type Config struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Options map[string]interface{} `json:"options"`
}

type CreateFunc func(name string, cfg map[string]any) (Conn, error)

var registry map[string]CreateFunc = make(map[string]CreateFunc)

func Register(typ string, f CreateFunc) {
	if _, ok := registry[typ]; ok {
		slog.Error("Duplicate register connector", "type", typ)
		os.Exit(1)
	}
	registry[typ] = f
	slog.Info("Rule connector registered", "type", typ)
}

func New(cfg Config) (Conn, error) {
	f, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("connector not found")
	}
	return f(cfg.Name, cfg.Options)
}
