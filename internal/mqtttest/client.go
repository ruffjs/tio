package mqtttest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type DeviceClient struct {
	opts      *mqtt.ClientOptions
	client    mqtt.Client
	onConnect func()
}

func NewDeviceClient(brokerURL, clientID, username, password string) *DeviceClient {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetUsername(username).
		SetPassword(password).
		SetAutoReconnect(true).
		SetCleanSession(true)

	return &DeviceClient{opts: opts}
}

func NewDeviceClientWithTLS(brokerURL, clientID, username, password, caFile, certFile, keyFile string, serverName ...string) (*DeviceClient, error) {
	tlsConfig := &tls.Config{}
	if len(serverName) > 0 {
		tlsConfig.ServerName = serverName[0]
	}

	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caFile)
		}
		tlsConfig.RootCAs = pool
	}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetUsername(username).
		SetPassword(password).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetTLSConfig(tlsConfig)

	return &DeviceClient{opts: opts}, nil
}

func (d *DeviceClient) OnConnect(cb func()) {
	d.onConnect = cb
}

func (d *DeviceClient) Connect(ctx context.Context) error {
	d.opts.SetDefaultPublishHandler(func(c mqtt.Client, m mqtt.Message) {
		slog.Debug("received message", "topic", m.Topic())
	})
	d.opts.OnConnect = func(c mqtt.Client) {
		slog.Info("mqtt client connected", "clientID", d.opts.ClientID)
		if d.onConnect != nil {
			go d.onConnect()
		}
	}
	d.opts.OnConnectionLost = func(c mqtt.Client, err error) {
		slog.Warn("mqtt client connection lost", "clientID", d.opts.ClientID, "error", err)
	}

	d.client = mqtt.NewClient(d.opts)
	token := d.client.Connect()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-token.Done():
		return token.Error()
	}
}

func (d *DeviceClient) Subscribe(topic string, qos byte, handler mqtt.MessageHandler) error {
	token := d.client.Subscribe(topic, qos, handler)
	token.Wait()
	return token.Error()
}

func (d *DeviceClient) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	token := d.client.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

func (d *DeviceClient) Disconnect() {
	if d.client != nil {
		d.client.Disconnect(250)
	}
}
