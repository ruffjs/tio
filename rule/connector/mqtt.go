package connector

import (
	"context"
	"log/slog"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/client"
)

// Mqtt connector

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, newMqtt)
}

type Mqtt struct {
	name   string
	config config.MqttClientConfig
	client client.Client
}

func newMqtt(name string, cfg map[string]any) (Conn, error) {
	var ac config.MqttClientConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		slog.Error("Rule connector mqtt failed to decode config", "error", err)
		os.Exit(1)
	}
	c := &Mqtt{
		name:   name,
		config: ac,
		client: client.NewClient(ac),
	}
	err := c.client.Connect(context.TODO())
	if err != nil {
		slog.Error("Rule connector mqtt connect failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Rule connector Mqtt inited")
	return c, nil
}

func (c *Mqtt) Close() error {
	c.client.Disconnect()
	return nil
}

func (c *Mqtt) Status() Status {
	// TODO
	return StatusConnected
}

func (c *Mqtt) Name() string {
	return c.name
}

func (*Mqtt) Type() string {
	return TypeMqtt
}

func (c *Mqtt) Connect() error {
	return c.client.Connect(context.TODO())
}

func (c *Mqtt) Conn() client.Client {
	return c.client
}

func (c *Mqtt) Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	return c.client.Subscribe(ctx, topic, qos, callback)
}

func (c *Mqtt) UnSubscribe(ctx context.Context, topic string) error {
	return c.client.Unsubscribe(ctx, topic)
}

func (c *Mqtt) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	tk := c.client.Publish(topic, qos, retained, payload)
	tk.Wait()
	return tk.Error()
}
