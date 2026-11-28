// Package client provides a small paho MQTT client for rule external MQTT connector.
package client

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"ruff.io/tio/config"
)

const clientCloseWaitMs = 50

type Client interface {
	Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error
	Unsubscribe(ctx context.Context, topic string) error
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
	Connect(ctx context.Context) error
	Disconnect()
	OnConnect(func())
}

type subscriber struct {
	ctx      context.Context
	topic    string
	qos      byte
	callback mqtt.MessageHandler
}

type mqttClient struct {
	sync.Mutex
	conn       mqtt.Client
	subscribes []subscriber
	onConnect  func()
}

var _ Client = (*mqttClient)(nil)

func NewClient(cfg config.MqttClientConfig) Client {
	slog.Info("Init mqtt client", "host", cfg.Host, "port", cfg.Port, "clientId", cfg.ClientId, "user", cfg.User)
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port)).
		SetClientID(cfg.ClientId).
		SetUsername(cfg.User).
		SetPassword(cfg.Password).
		SetAutoReconnect(true)
	cleanSession := true
	if cfg.CleanSession != nil {
		cleanSession = *cfg.CleanSession
	}
	opts.SetCleanSession(cleanSession)

	if cfg.WillTopic != "" && cfg.WillPayload != "" {
		opts.SetWill(cfg.WillTopic, cfg.WillPayload, 1, true)
	}

	var client mqttClient

	opts.SetDefaultPublishHandler(func(_ mqtt.Client, msg mqtt.Message) {
		slog.Debug("Received mqtt message", "topic", msg.Topic())
	})

	opts.OnConnect = func(_ mqtt.Client) {
		slog.Info("Mqtt client connected", "clientId", cfg.ClientId, "user", cfg.User)
		for _, s := range client.subscribes {
			if err := client.subscribe(s.ctx, s.topic, s.qos, s.callback); err != nil {
				slog.Error("Failed subscribe on reconnect", "topic", s.topic, "error", err)
			} else {
				slog.Info("Subscribe topic success on reconnect", "topic", s.topic)
			}
		}
		if client.onConnect != nil {
			go client.onConnect()
		}
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		slog.Warn("Mqtt client connection lost", "clientId", cfg.ClientId, "user", cfg.User, "error", err)
	}

	client = mqttClient{conn: mqtt.NewClient(opts)}
	return &client
}

func (c *mqttClient) Connect(ctx context.Context) error {
	if c.conn.IsConnected() {
		return nil
	}
	slog.Info("Mqtt client connecting")
	if token := c.conn.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	go func() {
		<-ctx.Done()
		c.conn.Disconnect(1000)
		slog.Info("Mqtt client disconnected cause context done")
	}()
	return nil
}

func (c *mqttClient) Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	c.Lock()
	defer c.Unlock()
	if c.conn.IsConnected() {
		if err := c.subscribe(ctx, topic, qos, callback); err != nil {
			return err
		}
	} else {
		c.conn.AddRoute(topic, callback)
	}
	c.subscribes = append(c.subscribes, subscriber{ctx, topic, qos, callback})
	slog.Debug("Added mqtt subscriber", "topic", topic, "qos", qos)
	return nil
}

func (c *mqttClient) subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	slog.Info("Subscribe mqtt topic", "topic", topic)
	token := c.conn.Subscribe(topic, qos, callback)
	select {
	case <-ctx.Done():
		slog.Debug("Give up subscribe cause context done", "topic", topic)
		return nil
	case <-token.Done():
		return token.Error()
	}
}

func (c *mqttClient) Unsubscribe(ctx context.Context, topic string) error {
	token := c.conn.Unsubscribe(topic)
	select {
	case <-ctx.Done():
		slog.Debug("Give up unsubscribe cause context done", "topic", topic)
		return nil
	case <-token.Done():
		if token.Error() != nil {
			return token.Error()
		}
		c.Lock()
		defer c.Unlock()
		index := -1
		for i, s := range c.subscribes {
			if s.topic == topic {
				index = i
				break
			}
		}
		if index >= 0 {
			c.subscribes = append(c.subscribes[:index], c.subscribes[index+1:]...)
		}
		return nil
	}
}

func (c *mqttClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	return c.conn.Publish(topic, qos, retained, payload)
}

func (c *mqttClient) Disconnect() {
	c.conn.Disconnect(clientCloseWaitMs)
}

func (c *mqttClient) OnConnect(callback func()) {
	c.onConnect = callback
}

func IsSysClient(id string) bool {
	return strings.HasPrefix(id, "$")
}
