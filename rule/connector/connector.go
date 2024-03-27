package connector

import (
	"fmt"
	"log/slog"
	"os"
)

type Status string

const (
	StatusConnecting   = "connecting"
	StatusConnected    = "connected"
	StatusDisconnected = "disconnected"
)

type Conn interface {
	Name() string
	Type() string
	Connect() error
	Status() Status
}

type Config struct {
	Name    string
	Type    string
	Options map[string]any
}

type CreateFunc func(name string, cfg map[string]any) Conn

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
	return f(cfg.Name, cfg.Options), nil
}
