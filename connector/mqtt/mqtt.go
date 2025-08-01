package mqtt

import (
	"context"
	"log"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"

	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/connector/mqtt/emqx"
)

const (
	DefaultQos = byte(1)
)

type mqttConnector struct {
	client client.Client
	connector.Connectivity
}

func (m mqttConnector) Subscribe(ctx context.Context, topic string, qos byte, callback func(msg connector.Message)) error {
	err := m.client.Subscribe(ctx, topic, qos, func(c mqtt.Client, message mqtt.Message) {
		callback(message)
	})
	return err
}

func (m mqttConnector) Publish(topic string, qos byte, retained bool, payload []byte) error {
	tk := m.client.Publish(topic, qos, retained, payload)
	select {
	case <-time.After(time.Second):
		return errors.New("timeout")
	case <-tk.Done():
		return tk.Error()
	}
}

var onceNewConnector sync.Once
var connectorSingleton connector.Connector

func InitConnector(cfg config.Connector, cl client.Client) connector.Connector {
	var c connector.Connectivity
	typ := cfg.Typ
	switch typ {
	case config.ConnectorMqttEmbed:
		c = embed.NewEmbedAdapter()
		slog.Info("Use embed connector")
	case config.ConnectorEmqx:
		c = emqx.NewEmqxAdapter(cfg.Emqx, cl)
		slog.Info("Use emqx connector")
	default:
		log.Fatalf("Unsupported connector type %s", typ)
	}

	onceNewConnector.Do(func() {
		connectorSingleton = &mqttConnector{cl, c}
	})
	return connectorSingleton
}

func Connector() connector.Connector { return connectorSingleton }

var _ connector.Connector = (*mqttConnector)(nil)
