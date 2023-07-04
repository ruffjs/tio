// Package client tio interacts with things by the mqtt client which connect to the mqtt broker
package client

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"ruff.io/tio/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"ruff.io/tio/pkg/log"
)

const (
	clientCloseWaitMs = 50

	SysClientIdPrefix = "$" // In order to distinguish the clientId of things and system
)

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
	log.Infof("Init mqtt client %#v", cfg)
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Host, cfg.Port))
	opts.SetClientID(cfg.ClientId)
	opts.SetUsername(cfg.User)
	opts.SetPassword(cfg.Password)
	cleanSession := false
	if cfg.CleanSession != nil {
		cleanSession = *cfg.CleanSession
	}
	opts.SetCleanSession(cleanSession)

	if cfg.WillTopic != "" && cfg.WillPayload != "" {
		opts.SetWill(cfg.WillTopic, cfg.WillPayload, 1, true)
	}

	var client mqttClient

	opts.SetDefaultPublishHandler(messagePubHandler)

	opts.OnConnect = func(c mqtt.Client) {
		log.Infof("Mqtt client connected, clientId: %s, user: %s", cfg.ClientId, cfg.User)
		for _, s := range client.subscribes {
			err := client.subscribe(s.ctx, s.topic, s.qos, s.callback)
			if err != nil {
				log.Errorf("Failed subscribe for topic %s at client connected event", s.topic)
			} else {
				log.Infof("Subscribe topic %s success at client connected event", s.topic)
			}
		}
		if client.onConnect != nil {
			go client.onConnect()
		}
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		log.Warnf("Mqtt client Connect lost, clientId: %s, user: %s, error: %v", cfg.ClientId, cfg.User, err)
	}

	client = mqttClient{conn: mqtt.NewClient(opts)}

	return &client
}

func (c *mqttClient) Connect(ctx context.Context) error {
	log.Infof("Mqtt client connecting ...")
	if token := c.conn.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	go func() {
		<-ctx.Done()
		c.conn.Disconnect(1000)
		log.Info("Mqtt client disconnected cause context done")
	}()
	return nil
}

// Subscribe retry subscribe when reconnected
// Important: MUST subscribe before mqtt client connected, for retain message or cached session message to process
func (c *mqttClient) Subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	// add to subscribes for retry subscribe when reconnected
	c.Lock()
	defer c.Unlock()
	if c.conn.IsConnected() {
		err := c.subscribe(ctx, topic, qos, callback)
		if err != nil {
			return err
		}
	} else {
		// For retained messages and cached session messages
		c.conn.AddRoute(topic, callback)
	}
	c.subscribes = append(c.subscribes, subscriber{ctx, topic, qos, callback})
	log.Debugf("Added subscriber topic=%s qos=%d", topic, qos)
	return nil
}

func (c *mqttClient) subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	log.Infof("Subscribe topic %s", topic)
	token := c.conn.Subscribe(topic, qos, callback)
	select {
	case <-ctx.Done():
		log.Debugf("Give up subscribe topic %s cause context done", topic)
		return nil
	case <-token.Done():
		return token.Error()
	}
}

func (c *mqttClient) Unsubscribe(ctx context.Context, topic string) error {
	token := c.conn.Unsubscribe(topic)
	select {
	case <-ctx.Done():
		log.Debugf("Give up unsubscribe topic %s cause context done", topic)
		return nil
	case <-token.Done():
		if token.Error() != nil {
			return token.Error()
		} else {
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
			} else {
				log.Errorf("Unsubscribe topic %s failed cause not found", topic)
			}
			return nil
		}
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

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Debugf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

func IsSysClient(id string) bool {
	return strings.HasPrefix(id, SysClientIdPrefix)
}
