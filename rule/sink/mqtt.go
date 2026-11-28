package sink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/go-viper/mapstructure/v2"
	"ruff.io/tio/connector"
	ruleconnector "ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, NewMqtt)
}

type MqttConfig struct {
	Topic    string `json:"topic"`
	Qos      byte   `json:"qos"`
	Retained bool   `json:"retained"`
}

func NewMqtt(ctx context.Context, name string, cfg map[string]any, ruleConn ruleconnector.Conn, _ connector.Connector) (Sink, error) {
	var ac MqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink Mqtt config", "name", name, "error", err)
		return nil, fmt.Errorf("decode sink Mqtt config: %w", err)
	}
	mqttConn, ok := ruleConn.(*ruleconnector.Mqtt)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for MQTT sink")
	}

	a := &MqttImpl{
		ctx:  ctx,
		name: name,
		cfg:  ac,
		conn: mqttConn,
	}
	return a, nil
}

type MqttImpl struct {
	ctx  context.Context
	name string
	cfg  MqttConfig
	conn *ruleconnector.Mqtt
	ch   chan *Msg
	done chan struct{}

	mu      sync.Mutex
	started bool
}

func (s *MqttImpl) Status() model.StatusInfo {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *MqttImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		slog.Info("Rule skip stopping sink (not started)", "type", TypeMqtt, "name", s.name)
		return nil
	}
	s.started = false
	close(s.done)
	slog.Info("Rule stopped sink", "type", TypeMqtt, "name", s.name)
	return nil
}

func (s *MqttImpl) Name() string { return s.name }

func (*MqttImpl) Type() string { return TypeMqtt }

func (s *MqttImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		slog.Info("Rule skip starting sink (already started)", "type", TypeMqtt, "name", s.name)
		return nil
	}
	s.ch = make(chan *Msg, 10000)
	s.done = make(chan struct{})
	s.started = true
	go s.publishLoop(s.ch, s.done)
	slog.Info("Rule started sink", "type", TypeMqtt, "name", s.name)
	return nil
}

func (s *MqttImpl) Publish(msg Msg) {
	s.mu.Lock()
	started := s.started
	ch, done := s.ch, s.done
	s.mu.Unlock()
	if !started {
		return
	}
	select {
	case ch <- &msg:
	case <-done:
	}
}

func (s *MqttImpl) publishLoop(ch chan *Msg, done chan struct{}) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-done:
			return
		case msg := <-ch:
			s.publish(msg)
		}
	}
}

func (s *MqttImpl) publish(msg *Msg) {
	topic := s.cfg.Topic
	if strings.Contains(s.cfg.Topic, "${thingId}") {
		if msg.ThingId == "" {
			slog.Error("Rule sink Mqtt send data missing thingId", "name", s.name, "topic", topic)
			return
		}
		topic = strings.ReplaceAll(s.cfg.Topic, "${thingId}", msg.ThingId)
	}
	accept, err := s.conn.Publish(topic, s.cfg.Qos, s.cfg.Retained, msg.Payload)
	if err != nil {
		slog.Error("Rule sink Mqtt send data", "name", s.name, "error", err)
	} else if accept {
		slog.Debug("Rule sink Mqtt send data SUCCESS", "name", s.name)
	}
}
