package sink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-viper/mapstructure/v2"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

const TypeEmbedMqtt = "embed-mqtt"

func init() {
	Register(TypeEmbedMqtt, NewEmbedMqtt)
}

type EmbedMqttConfig struct {
	Topic    string `json:"topic"`
	Qos      byte   `json:"qos"`
	Retained bool   `json:"retained"`
}

func NewEmbedMqtt(_ context.Context, name string, cfg map[string]any, _ ruleconnector.Conn, mainConn connector.Connector) (Sink, error) {
	var ac EmbedMqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	a := &embedMqttImpl{
		name: name,
		cfg:  ac,
		conn: mainConn,
	}
	return a, nil
}

type embedMqttImpl struct {
	name string
	cfg  EmbedMqttConfig
	conn connector.Connector
	ch   chan *Msg
	done chan struct{}

	started atomic.Bool
	mu      sync.Mutex
}

func (s *embedMqttImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started.Load() {
		return nil
	}
	s.ch = make(chan *Msg, 10000)
	s.done = make(chan struct{})
	s.started.Store(true)
	go s.publishLoop(s.ch, s.done)
	slog.Info("Rule started sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *embedMqttImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started.Load() {
		return nil
	}
	s.started.Store(false)
	close(s.done)
	slog.Info("Rule stopped sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *embedMqttImpl) Status() model.StatusInfo {
	if !s.started.Load() {
		return model.StatusNotStarted()
	}
	return model.StatusConnected()
}

func (s *embedMqttImpl) Name() string { return s.name }

func (*embedMqttImpl) Type() string { return TypeEmbedMqtt }

func (s *embedMqttImpl) Publish(msg Msg) {
	if !s.started.Load() {
		return
	}
	s.mu.Lock()
	ch, done := s.ch, s.done
	s.mu.Unlock()
	select {
	case ch <- &msg:
	case <-done:
	}
}

func (s *embedMqttImpl) publishLoop(ch chan *Msg, done chan struct{}) {
	for {
		select {
		case msg := <-ch:
			s.publish(msg)
		case <-done:
			return
		}
	}
}

func (s *embedMqttImpl) publish(msg *Msg) {
	topic := s.cfg.Topic
	if strings.Contains(topic, "${thingId}") {
		if msg.ThingId == "" {
			slog.Error("Rule sink embed-mqtt topic contains ${thingId} but msg.ThingId is empty", "name", s.name, "topic", topic)
			return
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
		slog.Error("Rule sink embed-mqtt publish failed", "name", s.name, "topic", topic, "error", err)
	}
}
