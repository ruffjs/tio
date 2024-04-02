package source

import (
	"log/slog"
	"os"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/rule/connector"
)

const TypeEmbedMqtt = "embed-mqtt"

func init() {
	Register(TypeEmbedMqtt, func(name string, cfg map[string]any, conn connector.Conn) Source {
		var ac EmbedMqttConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("decode source embed-mqtt config", "name", name, "error", err)
			os.Exit(1)
		}
		return NewEmbedMqtt(name, ac)
	})
}

type EmbedMqttConfig struct {
	Topic string
}

func NewEmbedMqtt(name string, cfg EmbedMqttConfig) Source {
	m := &embedMqttImpl{
		name:   name,
		config: cfg,
	}
	return m
}

type embedMqttImpl struct {
	name           string
	config         EmbedMqttConfig
	subscriptionId int
	handlers       []MsgHander
}

func (m *embedMqttImpl) Start() {
	m.sub()
}

func (m *embedMqttImpl) Stop() {
	if m.subscriptionId > 0 {
		err := embed.BrokerInstance().Unsubscribe(m.config.Topic, m.subscriptionId)
		if err != nil {
			slog.Error("Rule source embed-mqtt stop, failed to unsubscribe", "name", m.name, "error", err)
		}
	}
}

func (m *embedMqttImpl) Name() string {
	return m.name
}

func (*embedMqttImpl) Type() string {
	return TypeEmbedMqtt
}

func (m *embedMqttImpl) OnMsg(h MsgHander) {
	m.handlers = append(m.handlers, h)
}

func (m *embedMqttImpl) sub() {
	subId, err := embed.BrokerInstance().Subscribe(m.config.Topic, func(msg embed.Msg) {
		mm := Msg{
			ThingId: msg.ThingId,
			Topic:   msg.Topic,
			Payload: string(msg.Payload),
		}
		for _, h := range m.handlers {
			h(mm)
		}
	})
	m.subscriptionId = subId
	if err != nil {
		slog.Error("Rule source embed-mqtt subscribe failed", "name", m.name, "topic", m.config.Topic)
		os.Exit(1)
	}
}
