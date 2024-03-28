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
	m.sub()
	return m
}

type embedMqttImpl struct {
	name     string
	config   EmbedMqttConfig
	handlers []MsgHander
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
	embed.BrokerInstance().Subscribe(m.config.Topic, func(msg embed.Msg) {
		mm := Msg{
			ThingId: msg.ThingId,
			Topic:   msg.Topic,
			Payload: string(msg.Payload),
		}
		for _, h := range m.handlers {
			h(mm)
		}
	})
}
