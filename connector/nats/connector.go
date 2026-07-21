package nats

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"

	"github.com/nats-io/nats.go"
	server "github.com/nats-io/nats-server/v2/server"
)

type Connector struct {
	cfg        config.NatsConfig
	natsSvr    *NatsServer
	auth       *NatsAuthenticator
	mqttPub    *mqttPublisher

	natsConn   *nats.Conn
	sysConn    *nats.Conn
	js         nats.JetStreamContext

	ctx        context.Context
	cancel     context.CancelFunc

	configured bool
	started    bool
	mu         sync.Mutex
}

func NewConnector(cfg config.NatsConfig) (*Connector, error) {
	return &Connector{cfg: cfg}, nil
}

func (c *Connector) ConfigureAuth(authzFn connector.AuthzFn, aclFn connector.AclFn) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.configured {
		return errors.New("connector already configured")
	}

	c.auth = NewNatsAuthenticator(
		authzFn,
		aclFn,
		c.cfg.SuperUsers,
		c.cfg.AppClient,
		c.cfg.SystemClient,
		c.cfg.MqttPublisher,
	)
	c.configured = true
	return nil
}

func (c *Connector) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.configured {
		return errors.New("connector not configured: call ConfigureAuth first")
	}
	if c.started {
		return errors.New("connector already started")
	}

	appAcc, err := c.startServer()
	if err != nil {
		return err
	}
	sysAcc, _ := c.natsSvr.Server().LookupAccount(SysAccountName)
	c.auth.SetAccounts(appAcc, sysAcc)

	if err := c.connectClients(ctx); err != nil {
		c.cleanupLocked()
		return err
	}

	if err := c.startMqttPublisher(ctx); err != nil {
		c.cleanupLocked()
		return err
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.started = true
	return nil
}

func (c *Connector) startServer() (*server.Account, error) {
	svr, err := StartNatsServer(c.cfg.Server, c.auth)
	if err != nil {
		return nil, fmt.Errorf("start nats server: %w", err)
	}
	c.natsSvr = svr
	appAcc, err := svr.Server().LookupAccount(AppAccountName)
	if err != nil {
		return nil, fmt.Errorf("lookup APP account: %w", err)
	}
	return appAcc, nil
}

func (c *Connector) connectClients(ctx context.Context) error {
	appConn, err := nats.Connect(
		c.natsSvr.ClientURL(),
		nats.UserInfo(c.cfg.AppClient.User, c.cfg.AppClient.Password),
		nats.Name("tio-app-internal"),
	)
	if err != nil {
		return fmt.Errorf("connect app client: %w", err)
	}
	c.natsConn = appConn

	sysConn, err := nats.Connect(
		c.natsSvr.ClientURL(),
		nats.UserInfo(c.cfg.SystemClient.User, c.cfg.SystemClient.Password),
		nats.Name("tio-sys-internal"),
	)
	if err != nil {
		appConn.Close()
		return fmt.Errorf("connect sys client: %w", err)
	}
	c.sysConn = sysConn

	js, err := appConn.JetStream()
	if err != nil {
		sysConn.Close()
		appConn.Close()
		return fmt.Errorf("get jetstream context: %w", err)
	}
	c.js = js
	return nil
}

func (c *Connector) startMqttPublisher(ctx context.Context) error {
	v, err := c.natsSvr.Server().Varz(nil)
	if err != nil {
		return fmt.Errorf("get server varz: %w", err)
	}
	mqttPort := v.MQTT.Port
	if mqttPort <= 0 {
		return fmt.Errorf("MQTT gateway port not available")
	}
	pub := newMqttPublisher(
		c.cfg.Server.ServerName,
		mqttPort,
		c.cfg.MqttPublisher.User,
		c.cfg.MqttPublisher.Password,
	)
	if err := pub.Connect(ctx); err != nil {
		return fmt.Errorf("connect mqtt publisher: %w", err)
	}
	c.mqttPub = pub
	return nil
}

func (c *Connector) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cleanupLocked()
}

func (c *Connector) cleanupLocked() error {
	var firstErr error
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.natsConn != nil {
		c.natsConn.Drain()
		c.natsConn.Close()
		c.natsConn = nil
	}
	if c.sysConn != nil {
		c.sysConn.Drain()
		c.sysConn.Close()
		c.sysConn = nil
	}
	if c.mqttPub != nil {
		c.mqttPub.Disconnect()
		c.mqttPub = nil
	}
	if c.natsSvr != nil {
		c.natsSvr.Shutdown()
		c.natsSvr = nil
	}
	c.started = false
	return firstErr
}

func (c *Connector) Publish(topic string, payload []byte) error {
	subject, err := MqttPublishTopicToNatsSubject(topic)
	if err != nil {
		return err
	}
	return c.natsConn.Publish(subject, payload)
}

func (c *Connector) PublishReliable(topic string, payload []byte) error {
	return c.mqttPub.Publish(topic, 1, false, payload)
}

func (c *Connector) PublishRetained(topic string, payload []byte) error {
	return c.mqttPub.Publish(topic, 1, true, payload)
}

func (c *Connector) Server() *NatsServer              { return c.natsSvr }
func (c *Connector) AppConn() *nats.Conn              { return c.natsConn }
func (c *Connector) SysConn() *nats.Conn              { return c.sysConn }
func (c *Connector) JetStream() nats.JetStreamContext { return c.js }
