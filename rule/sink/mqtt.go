package sink

import (
	"log/slog"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
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

func NewMqtt(name string, cfg map[string]any, conn connector.Conn) Sink {
	var ac MqttConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("decode sink Mqtt config", "name", name, "error", err)
		os.Exit(1)
	}
	var mqttConn *connector.Mqtt
	if c, ok := conn.(*connector.Mqtt); !ok {
		slog.Error(("Rule source mqtt failed to cast connector to Mqtt"), "name", name)
		os.Exit(1)
	} else {
		mqttConn = c
	}

	a := &MqttImpl{
		name: name,
		cfg:  ac,
		conn: mqttConn,
		ch:   make(chan *Msg, 10000),
	}
	go a.publishLoop()
	return a
}

type MqttImpl struct {
	name string
	cfg  MqttConfig
	conn *connector.Mqtt
	ch   chan *Msg
}

func (s *MqttImpl) Name() string {
	return s.name
}

func (*MqttImpl) Type() string {
	return TypeMqtt
}

func (s *MqttImpl) Publish(msg Msg) {
	s.ch <- &msg
}

func (s *MqttImpl) publishLoop() {
	for {
		msg := <-s.ch
		topic := strings.ReplaceAll(s.cfg.Topic, "${thingId}", msg.ThingId)
		err := s.conn.Publish(topic, s.cfg.Qos, s.cfg.Retained, msg.Payload)

		if err != nil {
			slog.Error("Rule sinke Mqttsend data", "error", err)
		} else {
			slog.Debug("Rule sink Mqttsend data SUCCESS", "message", msg)
		}
	}
}
