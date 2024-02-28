package source

import (
	"ruff.io/tio/connector/mqtt/embed"
)

type EmbedMqttConfig struct {
	Topic string
}

func NewEmbedMqtt(cfg EmbedMqttConfig) Source {
	m := &embedMqttImpl{
		config: cfg,
	}
	m.sub()
	return m
}

type embedMqttImpl struct {
	config  EmbedMqttConfig
	handler MsgHander
}

func (*embedMqttImpl) Type() string {
	return TypeEmbedMqtt
}

func (m *embedMqttImpl) OnMsg(h MsgHander) {
	m.handler = h
}

func (m *embedMqttImpl) sub() {
	embed.BrokerInstance().Subscribe(m.config.Topic, func(msg embed.Msg) {
		mm := Msg{
			ThingId: msg.ThingId,
			Topic:   msg.Topic,
			Payload: msg.Payload,
		}
		if m.handler != nil {
			go m.handler(mm)
		}
	})
}
