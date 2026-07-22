package sink

import (
	"context"
	"fmt"
	"log/slog"

	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

type Msg struct {
	ThingId string `json:"thingId"`
	Topic   string `json:"topic"`
	Payload string `json:"payload"`
}
type Sink interface {
	Type() string
	Name() string
	Start() error
	Stop() error
	Status() model.StatusInfo
	Publish(msg Msg)
}

type Config struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Connector string         `json:"connector"`
	Enabled   bool           `json:"enabled"`
	Options   map[string]any `json:"options"`
}

type CreateFunc func(ctx context.Context, name string, cfg map[string]any, ruleConn ruleconnector.Conn, mainConn connector.Connector) (Sink, error)

type Metric struct {
	Received int64 `json:"received"`
	Success  int64 `json:"success"`
	Failed   int64 `json:"failed"`
}

var registry map[string]CreateFunc = make(map[string]CreateFunc)

func Register(typ string, f CreateFunc) {
	if _, ok := registry[typ]; ok {
		slog.Error("Duplicate register sink", "type", typ)
		return
	}
	registry[typ] = f
	slog.Info("Rule sink registered", "type", typ)
}

func New(ctx context.Context, cfg Config, ruleConn ruleconnector.Conn, mainConn connector.Connector) (Sink, error) {
	f, ok := registry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("sink not found")
	}
	return f(ctx, cfg.Name, cfg.Options, ruleConn, mainConn)
}

func withConnStatus(connName string, connStatus model.StatusInfo) model.StatusInfo {
	if connStatus.Status != model.Connected {
		st := model.StatusDisconnected(
			fmt.Sprintf("connector %q is not connected: %s", connName, connStatus.Reason),
			connStatus.Error)
		return st
	}
	return connStatus
}
