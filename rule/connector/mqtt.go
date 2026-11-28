package connector

import (
	"context"
	"log/slog"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-viper/mapstructure/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/rule/model"
)

const TypeMqtt = "mqtt"

func init() {
	Register(TypeMqtt, newMqtt)
}

type Mqtt struct {
	ctx    context.Context
	name   string
	config config.MqttClientConfig
	client client.Client

	mu      sync.RWMutex
	started bool
	status  model.StatusInfo
}

func newMqtt(ctx context.Context, name string, cfg map[string]any) (Conn, error) {
	var ac config.MqttClientConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, errors.WithMessage(err, "failed to decode config")
	}
	c := &Mqtt{
		ctx:    ctx,
		name:   name,
		config: ac,
		client: client.NewClient(ac),
		status: model.StatusNotStarted(),
	}
	slog.Info("Rule connector Mqtt inited", "name", name)
	return c, nil
}

func (c *Mqtt) Name() string { return c.name }

func (*Mqtt) Type() string { return TypeMqtt }

func (c *Mqtt) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil
	}
	c.started = true
	c.status = model.StatusConnecting()
	go func() {
		if err := c.client.Connect(c.ctx); err != nil {
			c.mu.Lock()
			c.status = model.StatusDisconnected(err.Error(), err)
			c.mu.Unlock()
		} else {
			c.mu.Lock()
			c.status = model.StatusConnected()
			c.mu.Unlock()
		}
	}()
	return nil
}

func (c *Mqtt) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.client.Disconnect()
	c.started = false
	c.status = model.StatusNotStarted()
	return nil
}

func (c *Mqtt) Status() model.StatusInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Mqtt) Conn() client.Client { return c.client }

func (c *Mqtt) Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	return c.client.Subscribe(ctx, topic, qos, callback)
}

func (c *Mqtt) UnSubscribe(ctx context.Context, topic string) error {
	return c.client.Unsubscribe(ctx, topic)
}

func (c *Mqtt) Publish(topic string, qos byte, retained bool, payload interface{}) (accept bool, err error) {
	c.mu.RLock()
	started := c.started
	c.mu.RUnlock()
	if !started {
		return false, nil
	}

	tk := c.client.Publish(topic, qos, retained, payload)
	select {
	case <-tk.Done():
	case <-c.ctx.Done():
		return true, errors.WithMessage(c.ctx.Err(), "interrupt publish cause context done")
	}
	return true, tk.Error()
}
