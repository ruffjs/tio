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

	mu          sync.RWMutex
	started     bool
	status      model.StatusInfo
	startCtx    context.Context
	startCancel context.CancelFunc
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
	c.client.OnConnect(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.started {
			c.status = model.StatusConnected()
		}
	})
	c.client.OnConnectionLost(func(err error) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.started {
			c.status = model.StatusDisconnected(err.Error(), err)
		}
	})
	slog.Info("Rule connector Mqtt inited", "name", name)
	return c, nil
}

func (c *Mqtt) Name() string { return c.name }

func (*Mqtt) Type() string { return TypeMqtt }

func (c *Mqtt) Start() error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	c.status = model.StatusConnecting()
	c.startCtx, c.startCancel = context.WithCancel(c.ctx)
	startCtx := c.startCtx
	c.mu.Unlock()

	go func() {
		err := c.client.Connect(startCtx)
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.started || startCtx.Err() != nil {
			c.client.Disconnect()
			if c.started {
				c.status = model.StatusNotStarted()
			}
			return
		}
		if err != nil {
			c.status = model.StatusDisconnected(err.Error(), err)
			return
		}
		c.status = model.StatusConnected()
	}()
	return nil
}

func (c *Mqtt) Stop() error {
	c.mu.Lock()
	if c.startCancel != nil {
		c.startCancel()
		c.startCancel = nil
	}
	c.started = false
	c.status = model.StatusNotStarted()
	c.mu.Unlock()

	c.client.Disconnect()
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
