package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	mqttPublishTimeout = 5 * time.Second
	mqttQuiesceMs      = 250
)

type mqttPublisher struct {
	client     mqtt.Client
	serverName string
	mqttPort   int
	user       string
	password   string
	connected  bool
	mu         sync.Mutex
}

func newMqttPublisher(serverName string, mqttPort int, user, password string) *mqttPublisher {
	return &mqttPublisher{
		serverName: serverName,
		mqttPort:   mqttPort,
		user:       user,
		password:   password,
	}
}

func (p *mqttPublisher) Connect(ctx context.Context) error {
	clientID := fmt.Sprintf("$tio-mqtt-pub-%s", p.serverName)

	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", p.mqttPort)).
		SetClientID(clientID).
		SetUsername(p.user).
		SetPassword(p.password).
		SetAutoReconnect(true).
		SetConnectRetryInterval(1 * time.Second)

	c := mqtt.NewClient(opts)
	token := c.Connect()

	select {
	case <-ctx.Done():
		c.Disconnect(mqttQuiesceMs)
		return ctx.Err()
	case <-token.Done():
		if token.Error() != nil {
			return fmt.Errorf("mqtt publisher connect: %w", token.Error())
		}
	}

	p.mu.Lock()
	p.client = c
	p.connected = true
	p.mu.Unlock()

	return nil
}

func (p *mqttPublisher) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if _, err := MqttPublishTopicToNatsSubject(topic); err != nil {
		return fmt.Errorf("invalid mqtt topic: %w", err)
	}

	p.mu.Lock()
	c := p.client
	p.mu.Unlock()

	if c == nil || !c.IsConnected() {
		return fmt.Errorf("mqtt publisher not connected")
	}

	token := c.Publish(topic, qos, retained, payload)

	if !token.WaitTimeout(mqttPublishTimeout) {
		return fmt.Errorf("mqtt publish timeout on topic %q", topic)
	}
	if token.Error() != nil {
		return fmt.Errorf("mqtt publish: %w", token.Error())
	}

	return nil
}

func (p *mqttPublisher) Disconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.client != nil {
		p.client.Disconnect(mqttQuiesceMs)
	}
	p.connected = false
}
