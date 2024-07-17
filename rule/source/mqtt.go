package source

import (
	"context"
	"log/slog"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/rule/connector"
)

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, func(name string, cfg map[string]any, conn connector.Conn) Source {
		var ac MqttConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("decode source embed-mqtt config", "name", name, "error", err)
			os.Exit(1)
		}
		var mqttConn *connector.Mqtt
		if c, ok := conn.(*connector.Mqtt); !ok {
			slog.Error(("Rule source mqtt failed to cast connector to Mqtt"), "name", name)
			os.Exit(1)
		} else {
			mqttConn = c
		}
		return NewMqtt(name, ac, mqttConn)
	})
}

type MqttConfig struct {
	Topic string
	Qos   byte
}

func NewMqtt(name string, cfg MqttConfig, conn *connector.Mqtt) Source {
	m := &mqttImpl{
		name:   name,
		config: cfg,
		conn:   conn,
	}
	return m
}

type mqttImpl struct {
	name     string
	config   MqttConfig
	conn     *connector.Mqtt
	handlers []MsgHander
}

func (m *mqttImpl) Start() {
	m.subscribe()
}

func (m *mqttImpl) Stop() {
	err := m.conn.UnSubscribe(context.TODO(), m.config.Topic)
	if err != nil {
		slog.Error("Rule source mqtt stop, failed to unsubscribe", "name", m.name, "error", err)
	}
}

func (m *mqttImpl) Name() string {
	return m.name
}

func (*mqttImpl) Type() string {
	return TypeMqtt
}

func (m *mqttImpl) OnMsg(h MsgHander) {
	m.handlers = append(m.handlers, h)
}

func (m *mqttImpl) subscribe() {
	err := m.conn.Subscribe(context.TODO(), m.config.Topic, m.config.Qos, func(cl mqtt.Client, msg mqtt.Message) {
		thId, err := model.GetThingIdFromTopic(msg.Topic())
		if err != nil {
			slog.Error("Can't get thing id from topic in embed mqtt broker subscription", "error", err)
		}
		mm := Msg{
			ThingId: thId,
			Topic:   msg.Topic(),
			Payload: string(msg.Payload()),
		}
		for _, h := range m.handlers {
			h(mm)
		}
	})
	if err != nil {
		slog.Error("Rule source mqtt subscribe failed", "name", m.name, "topic", m.config.Topic)
		os.Exit(1)
	}
}
