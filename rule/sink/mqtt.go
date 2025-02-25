package sink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// Mqtt sink

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, NewMqtt)
}

type MqttConfig struct {
	Topic    string `json:"topic"`
	Qos      byte   `json:"qos"`
	Retained bool   `json:"retained"`
}

func NewMqtt(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Sink, error) {
	var ac MqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink Mqtt config", "name", name, "error", err)
		return nil, fmt.Errorf("decode sink Mqtt config: %w", err)
	}
	var mqttConn *connector.Mqtt
	if c, ok := conn.(*connector.Mqtt); !ok {
		return nil, fmt.Errorf("wrong connector type for MQTT sink")
	} else {
		mqttConn = c
	}

	a := &MqttImpl{
		ctx:  ctx,
		name: name,
		cfg:  ac,
		conn: mqttConn,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a, nil
}

type MqttImpl struct {
	ctx  context.Context
	name string
	cfg  MqttConfig
	conn *connector.Mqtt
	ch   chan *Msg

	started bool
}

func (s *MqttImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *MqttImpl) Stop() error {
	if !s.started {
		slog.Info("Rule skip stopping sink (not started)", "type", TypeMqtt, "name", s.name)
		return nil
	}
	s.started = false
	slog.Info("Rule stopped sink", "type", TypeMqtt, "name", s.name)
	return nil
}

func (s *MqttImpl) Name() string {
	return s.name
}

func (*MqttImpl) Type() string {
	return TypeMqtt
}

func (s *MqttImpl) Start() error {
	if s.started {
		slog.Info("Rule skip starting sink (already started)", "type", TypeMqtt, "name", s.name)
		return nil
	}
	s.started = true
	err := s.Status().Error
	if err == nil {
		slog.Info("Rule started sink", "type", TypeMqtt, "name", s.name)
	}
	return err
}

func (s *MqttImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *MqttImpl) publishLoop() {
	for {
		var msg *Msg
		select {
		case <-s.ctx.Done():
			return
		case msg = <-s.ch:
		}
		topic := s.cfg.Topic
		if strings.Contains(s.cfg.Topic, "${thingId}") {
			topic = strings.ReplaceAll(s.cfg.Topic, "${thingId}", msg.ThingId)
			if msg.ThingId == "" {
				slog.Error("Rule sink Mqtt send data", "name", s.name, "error", "missing thingId", "message", msg)
				return
			}
		}
		accept, err := s.conn.Publish(topic, s.cfg.Qos, s.cfg.Retained, msg.Payload)

		if err != nil {
			slog.Error("Rule sinke Mqtt send data", "name", s.name, "error", err)
		} else if accept {
			slog.Debug("Rule sink Mqtt send data SUCCESS", "name", s.name, "message", msg)
		}
	}
}
