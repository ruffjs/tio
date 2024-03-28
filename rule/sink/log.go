package sink

import (
	"encoding/json"
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	"ruff.io/tio/rule/connector"
)

// Log sink, for debug

const TypeLog = "log"

func init() {
	Register(TypeLog, NewLog)
}

func NewLog(name string, cfg map[string]any, conn connector.Conn) Sink {
	a := &logImpl{
		name: name,
		ch:   make(chan *Msg, 100),
	}
	go a.publishLoop()
	return a
}

type logImpl struct {
	name string
	ch   chan *Msg
}

func (s *logImpl) Name() string {
	return s.name
}

func (*logImpl) Type() string {
	return TypeLog
}

func (s *logImpl) Publish(msg Msg) {
	s.ch <- &msg
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
