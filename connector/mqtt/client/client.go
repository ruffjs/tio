// Package client tio interacts with things by the mqtt client which connect to the mqtt broker
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"ruff.io/tio/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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
	slog.Info("Init mqtt client", slog.Any("config", cfg))

	// Build broker URL with appropriate protocol
	protocol := "tcp"
	if cfg.TLS != nil {
		protocol = "ssl"
	}

	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("%s://%s:%d", protocol, cfg.Host, cfg.Port)).
		SetClientID(cfg.ClientId).
		SetUsername(cfg.User).
		SetPassword(cfg.Password).
		SetAutoReconnect(true)

	// Configure TLS if provided
	if cfg.TLS != nil {
		tlsConfig := &tls.Config{
			ServerName:         cfg.TLS.ServerName,
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}

		// Load CA certificate if provided
		if cfg.TLS.CAFile != "" {
			caCert, err := os.ReadFile(cfg.TLS.CAFile)
			if err != nil {
				slog.Error("Failed to read CA file", slog.String("file", cfg.TLS.CAFile), slog.Any("error", err))
			} else {
				caCertPool := x509.NewCertPool()
				if !caCertPool.AppendCertsFromPEM(caCert) {
					slog.Error("Failed to parse CA certificate")
				} else {
					tlsConfig.RootCAs = caCertPool
				}
			}
		}

		// Load client certificate and key for mutual TLS
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			if err != nil {
				slog.Error("Failed to load client certificate", slog.String("cert", cfg.TLS.CertFile), slog.Any("error", err))
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		opts.SetTLSConfig(tlsConfig)
	}

	cleanSession := true
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
		slog.Info("Mqtt client connected", slog.String("clientId", cfg.ClientId), slog.String("user", cfg.User))
		for _, s := range client.subscribes {
			err := client.subscribe(s.ctx, s.topic, s.qos, s.callback)
			if err != nil {
				slog.Error("Failed subscribe for topic", slog.String("topic", s.topic), slog.Any("error", err))
			} else {
				slog.Info("Subscribe topic success", slog.String("topic", s.topic))
			}
		}
		if client.onConnect != nil {
			go client.onConnect()
		}
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		slog.Warn("Mqtt client Connect lost", slog.String("clientId", cfg.ClientId), slog.String("user", cfg.User), slog.Any("error", err))
	}

	client = mqttClient{conn: mqtt.NewClient(opts)}

	return &client
}

func (c *mqttClient) Connect(ctx context.Context) error {
	if c.conn.IsConnected() {
		return nil
	}
	slog.Info("Mqtt client connecting ...")
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
	slog.Debug("Added subscriber", slog.String("topic", topic), slog.Int("qos", int(qos)))
	return nil
}

func (c *mqttClient) subscribe(ctx context.Context, topic string, qos byte, callback mqtt.MessageHandler) error {
	slog.Info("Subscribe topic", slog.String("topic", topic))
	token := c.conn.Subscribe(topic, qos, callback)
	select {
	case <-ctx.Done():
		slog.Debug("Give up subscribe topic", slog.String("topic", topic), slog.Any("cause", ctx.Err()))
		return nil
	case <-token.Done():
		return token.Error()
	}
}

func (c *mqttClient) Unsubscribe(ctx context.Context, topic string) error {
	token := c.conn.Unsubscribe(topic)
	select {
	case <-ctx.Done():
		slog.Debug("Give up unsubscribe topic", slog.String("topic", topic), slog.Any("cause", ctx.Err()))
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
				slog.Error("Unsubscribe topic failed cause not found", slog.String("topic", topic))
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
	slog.Debug("Received message", slog.String("topic", msg.Topic()), slog.Any("payload", msg.Payload()))
}

func IsSysClient(id string) bool {
	return strings.HasPrefix(id, SysClientIdPrefix)
}
