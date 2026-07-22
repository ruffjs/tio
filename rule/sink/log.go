package sink

import (
	"context"
	"encoding/json"
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// Log sink, for debug

const TypeLog = "log"

func init() {
	Register(TypeLog, NewLog)
}

func NewLog(ctx context.Context, name string, cfg map[string]any, _ ruleconnector.Conn, _ connector.Connector) (Sink, error) {
	a := &logImpl{
		name: name,
		ch:   make(chan *Msg, 100),
	}
	go a.publishLoop()
	return a, nil
}

type logImpl struct {
	name    string
	ch      chan *Msg
	started bool
}

func (s *logImpl) Start() error {
	s.started = true
	slog.Info("Rule started sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *logImpl) Status() model.StatusInfo {
	return model.StatusConnected()
}

func (s *logImpl) Stop() error {
	s.started = false
	slog.Info("Rule stopped sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *logImpl) Name() string {
	return s.name
}

func (*logImpl) Type() string {
	return TypeLog
}

func (s *logImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *logImpl) publishLoop() {
	for {
		msg := <-s.ch
		b, err := json.Marshal(msg)
		if err != nil {
			slog.Error("Rule Log sink marshal msg", "error", err)
			return
		}
		slog.Info("Rule Log sink", "message", b)
	}
}
