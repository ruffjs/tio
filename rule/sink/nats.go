package sink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

const TypeNats = "mqtt"

func init() {
	Register(TypeNats, NewNats)
}

type NatsConfig struct {
	Topic    string `json:"topic"`
	Qos      byte   `json:"qos"`
	Retained bool   `json:"retained"`
}

func NewNats(_ context.Context, name string, cfg map[string]any, _ ruleconnector.Conn, mainConn connector.Connector) (Sink, error) {
	var ac NatsConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	a := &natsImpl{
		name: name,
		cfg:  ac,
		conn: mainConn,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a, nil
}

type natsImpl struct {
	name string
	cfg  NatsConfig
	conn connector.Connector
	ch   chan *Msg

	started bool
	mu      sync.Mutex
}

func (s *natsImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = true
	slog.Info("Rule started sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *natsImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = false
	slog.Info("Rule stopped sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *natsImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return model.StatusConnected()
}

func (s *natsImpl) Name() string {
	return s.name
}

func (*natsImpl) Type() string {
	return TypeNats
}

func (s *natsImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *natsImpl) publishLoop() {
	for msg := range s.ch {
		topic := s.cfg.Topic
		if strings.Contains(topic, "${thingId}") {
			if msg.ThingId == "" {
				slog.Error("NATS sink topic contains ${thingId} but msg.ThingId is empty", "name", s.name, "topic", topic)
				continue
			}
			topic = strings.ReplaceAll(topic, "${thingId}", msg.ThingId)
		}

		payload := []byte(msg.Payload)
		var err error
		switch {
		case s.cfg.Retained:
			err = s.conn.PublishRetained(topic, payload)
		case s.cfg.Qos >= 1:
			err = s.conn.PublishReliable(topic, payload)
		default:
			err = s.conn.Publish(topic, payload)
		}
		if err != nil {
			slog.Error("NATS sink publish failed", "name", s.name, "topic", topic, "error", err)
		} else {
			slog.Debug("NATS sink published", "name", s.name, "topic", topic)
		}
	}
}
