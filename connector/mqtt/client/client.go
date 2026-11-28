// Package client provides a small paho MQTT client for rule external MQTT connector.
package client

import (
	"context"
	"fmt"
	"log/slog"
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
	OnConnectionLost(func(error))
}

type subscriber struct {
	ctx      context.Context
	callback mqtt.MessageHandler
}

type topicSub struct {
	qos        byte
	handlers   []subscriber
	subscribed bool
}

type mqttClient struct {
	mu         sync.RWMutex
	conn       mqtt.Client
	topics     map[string]*topicSub
	onConnect  func()
	onLost     func(error)
	disconnect context.CancelFunc
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

	c := &mqttClient{topics: make(map[string]*topicSub)}
	opts.SetDefaultPublishHandler(func(_ mqtt.Client, msg mqtt.Message) {
		slog.Debug("Received mqtt message", "topic", msg.Topic())
	})
	opts.OnConnect = func(_ mqtt.Client) {
		slog.Info("Mqtt client connected", "clientId", cfg.ClientId, "user", cfg.User)
		c.resubscribeAll()
		c.mu.RLock()
		cb := c.onConnect
		c.mu.RUnlock()
		if cb != nil {
			go cb()
		}
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		slog.Warn("Mqtt client connection lost", "clientId", cfg.ClientId, "user", cfg.User, "error", err)
		c.mu.Lock()
		for _, ts := range c.topics {
			ts.subscribed = false
		}
		cb := c.onLost
		c.mu.Unlock()
		if cb != nil {
			cb(err)
		}
	}
	c.conn = mqtt.NewClient(opts)
	return c
}

func (c *mqttClient) Connect(ctx context.Context) error {
	if c.conn.IsConnected() {
		return nil
	}
	waitCtx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.disconnect = cancel
	c.mu.Unlock()

	slog.Info("Mqtt client connecting")
	if token := c.conn.Connect(); token.Wait() && token.Error() != nil {
		cancel()
		return token.Error()
	}
	go func() {
		<-waitCtx.Done()
		c.conn.Disconnect(1000)
		slog.Info("Mqtt client disconnected cause context done")
	}()
	return nil
}

func (c *mqttClient) Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	c.mu.Lock()
	ts, ok := c.topics[topic]
	if !ok {
		ts = &topicSub{}
		c.topics[topic] = ts
	}
	if qos > ts.qos {
		ts.qos = qos
	}
	ts.handlers = append(ts.handlers, subscriber{ctx: ctx, callback: callback})
	needSub := !ts.subscribed && c.conn.IsConnected()
	subQos := ts.qos
	c.mu.Unlock()

	if needSub {
		if err := c.subscribeTopic(ctx, topic, subQos); err != nil {
			return err
		}
		c.mu.Lock()
		if cur, ok := c.topics[topic]; ok && len(cur.handlers) > 0 {
			cur.subscribed = true
		}
		c.mu.Unlock()
	}
	slog.Debug("Added mqtt subscriber", "topic", topic, "qos", qos)
	return nil
}

func (c *mqttClient) subscribeTopic(ctx context.Context, topic string, qos byte) error {
	slog.Info("Subscribe mqtt topic", "topic", topic)
	token := c.conn.Subscribe(topic, qos, c.dispatch(topic))
	select {
	case <-ctx.Done():
		slog.Debug("Give up subscribe cause context done", "topic", topic)
		return nil
	case <-token.Done():
		return token.Error()
	}
}

func (c *mqttClient) dispatch(topic string) mqtt.MessageHandler {
	return func(cl mqtt.Client, msg mqtt.Message) {
		c.mu.RLock()
		ts, ok := c.topics[topic]
		if !ok {
			c.mu.RUnlock()
			return
		}
		handlers := make([]subscriber, len(ts.handlers))
		copy(handlers, ts.handlers)
		c.mu.RUnlock()

		for _, h := range handlers {
			if h.ctx != nil && h.ctx.Err() != nil {
				continue
			}
			h.callback(cl, msg)
		}
	}
}

func (c *mqttClient) resubscribeAll() {
	c.mu.RLock()
	type sub struct {
		topic string
		qos   byte
	}
	var subs []sub
	for topic, ts := range c.topics {
		if len(ts.handlers) > 0 {
			subs = append(subs, sub{topic: topic, qos: ts.qos})
		}
	}
	c.mu.RUnlock()

	for _, s := range subs {
		if err := c.subscribeTopic(context.Background(), s.topic, s.qos); err != nil {
			slog.Error("Failed subscribe on reconnect", "topic", s.topic, "error", err)
			continue
		}
		c.mu.Lock()
		if ts, ok := c.topics[s.topic]; ok && len(ts.handlers) > 0 {
			ts.subscribed = true
		}
		c.mu.Unlock()
		slog.Info("Subscribe topic success on reconnect", "topic", s.topic)
	}
}

func (c *mqttClient) Unsubscribe(ctx context.Context, topic string) error {
	c.mu.Lock()
	ts, ok := c.topics[topic]
	if !ok || len(ts.handlers) == 0 {
		c.mu.Unlock()
		return nil
	}
	// remove handlers whose ctx is done, or all if called as full unsubscribe for topic
	remaining := ts.handlers[:0]
	for _, h := range ts.handlers {
		if h.ctx != nil && h.ctx.Err() != nil {
			continue
		}
		remaining = append(remaining, h)
	}
	// If caller wants full unsubscribe, drop all remaining handlers for this topic.
	ts.handlers = nil
	ts.subscribed = false
	delete(c.topics, topic)
	_ = remaining
	c.mu.Unlock()

	token := c.conn.Unsubscribe(topic)
	select {
	case <-ctx.Done():
		slog.Debug("Give up unsubscribe cause context done", "topic", topic)
		return nil
	case <-token.Done():
		return token.Error()
	}
}

func (c *mqttClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	return c.conn.Publish(topic, qos, retained, payload)
}

func (c *mqttClient) Disconnect() {
	c.mu.RLock()
	cancel := c.disconnect
	c.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	c.conn.Disconnect(clientCloseWaitMs)
}

func (c *mqttClient) OnConnect(callback func()) {
	c.mu.Lock()
	c.onConnect = callback
	c.mu.Unlock()
}

func (c *mqttClient) OnConnectionLost(callback func(error)) {
	c.mu.Lock()
	c.onLost = callback
	c.mu.Unlock()
}
