package sink

import (
	"log/slog"
	"os"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/rule/connector"
)

const TypeEmbedMqtt = "embed-mqtt"

func init() {
	Register(TypeEmbedMqtt, func(name string, cfg map[string]any, conn connector.Conn) Sink {
		var ac EmbedMqttConfig
		if err := mapstructure.Decode(cfg, &ac); err != nil {
			slog.Error("Rule decode sink embed-mqtt config", "name", name, "error", err)
			os.Exit(1)
		}
		return NewEmbedMqtt(name, ac)
	})
}

type EmbedMqttConfig struct {
	Topic    string `json:"topic"`
	Qos      byte   `json:"qos"`
	Retained bool   `json:"retained"`
}

func NewEmbedMqtt(name string, cfg EmbedMqttConfig) Sink {
	m := &embedMqttImpl{
		name:   name,
		config: cfg,
	}
	return m
}

type embedMqttImpl struct {
	name   string
	config EmbedMqttConfig
}

func (m *embedMqttImpl) Publish(msg Msg) {
	err := embed.BrokerInstance().Publish(m.config.Topic, []byte(msg.Payload), m.config.Retained, m.config.Qos)
	if err != nil {
		slog.Error("Rule sink embed-mqtt publish mesage", "name", m.name, "error", err, "message", msg)
	}
}

func (m *embedMqttImpl) Name() string {
	return m.name
}

func (*embedMqttImpl) Type() string {
	return TypeEmbedMqtt
}
