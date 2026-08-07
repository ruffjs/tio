package nats

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type Connector struct {
	cfg     config.NatsConfig
	natsSvr *NatsServer
	auth    *NatsAuthenticator
	mqttPub *mqttPublisher

	natsConn *nats.Conn
	sysConn  *nats.Conn
	js       nats.JetStreamContext
	kv       nats.KeyValue

	presenceHandler connector.PresenceHandler

	deviceCodec  codec.Codec
	protocolMode string

	ctx    context.Context
	cancel context.CancelFunc

	configured bool
	started    bool
	mu         sync.Mutex
}

func NewConnector(cfg config.NatsConfig, deviceCodec codec.Codec, protocolMode string) (*Connector, error) {
	if deviceCodec == nil {
		return nil, errors.New("device codec is required")
	}
	if protocolMode != "legacy" && protocolMode != "simple" {
		return nil, fmt.Errorf("invalid protocol mode: %q", protocolMode)
	}
	return &Connector{
		cfg:          cfg,
		deviceCodec:  deviceCodec,
		protocolMode: protocolMode,
	}, nil
}

func (c *Connector) ConfigureAuth(authzFn connector.AuthzFn, bg connector.BindingGetter) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.configured {
		return errors.New("connector already configured")
	}

	c.auth = NewNatsAuthenticator(
		authzFn,
		bg,
		c.cfg.SuperUsers,
		c.cfg.AppClient,
		c.cfg.SystemClient,
		c.cfg.MqttPublisher,
		c.protocolMode,
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
	if c.natsSvr != nil {
		return errors.New("server already started: use StartClients to continue")
	}

	appAcc, err := c.startServer()
	if err != nil {
		return err
	}
	sysAcc, _ := c.natsSvr.Server().LookupAccount(SysAccountName)
	c.auth.SetAccounts(appAcc, sysAcc)

	if err := c.startServerAndClients(ctx); err != nil {
		return err
	}

	c.started = true
	return nil
}

// StartServerOnly starts the embedded NATS server and configures the
// authenticator, but does not connect internal clients or start
// presence/control. This is intended for cluster testing where all
// servers must be running before MQTT clients can connect.
// StartClients must be called afterwards to complete startup.
func (c *Connector) StartServerOnly() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.configured {
		return errors.New("connector not configured: call ConfigureAuth first")
	}
	if c.started || c.natsSvr != nil {
		return errors.New("connector already started")
	}

	appAcc, err := c.startServer()
	if err != nil {
		return err
	}
	sysAcc, _ := c.natsSvr.Server().LookupAccount(SysAccountName)
	c.auth.SetAccounts(appAcc, sysAcc)
	return nil
}

// StartClients connects internal NATS/MQTT clients and starts presence
// and control subsystems. Must be called after StartServerOnly.
func (c *Connector) StartClients(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.configured {
		return errors.New("connector not configured: call ConfigureAuth first")
	}
	if c.started {
		return errors.New("connector already started")
	}
	if c.natsSvr == nil {
		return errors.New("server not started: call StartServerOnly first")
	}

	if err := c.startServerAndClients(ctx); err != nil {
		return err
	}

	c.started = true
	return nil
}

// startServerAndClients connects clients, starts MQTT publisher, and
// initializes presence/control. The server must already be started.
func (c *Connector) startServerAndClients(ctx context.Context) error {
	if err := c.connectClients(ctx); err != nil {
		c.cleanupLocked()
		return err
	}

	if err := c.startMqttPublisher(ctx); err != nil {
		c.cleanupLocked()
		return err
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	if err := c.initPresence(); err != nil {
		c.cleanupLocked()
		return err
	}

	if err := c.initControl(); err != nil {
		c.cleanupLocked()
		return err
	}
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

	var pubTLS *tls.Config
	if !tlsConfigEmpty(c.cfg.Server.MqttTLS) {
		tlsCfg, err := buildMqttPublisherTLSConfig(c.cfg.Server.MqttTLS)
		if err != nil {
			return fmt.Errorf("build MQTT publisher TLS config: %w", err)
		}
		pubTLS = tlsCfg
	}

	pub := newMqttPublisher(
		c.cfg.Server.ServerName,
		mqttPort,
		c.cfg.MqttPublisher.User,
		c.cfg.MqttPublisher.Password,
		pubTLS,
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
		if err := c.natsConn.Drain(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.natsConn.Close()
		c.natsConn = nil
	}
	if c.sysConn != nil {
		if err := c.sysConn.Drain(); err != nil && firstErr == nil {
			firstErr = err
		}
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
	return c.publishMqtt(topic, 1, false, payload)
}

func (c *Connector) PublishRetained(topic string, payload []byte) error {
	return c.publishMqtt(topic, 1, true, payload)
}

func (c *Connector) publishMqtt(topic string, qos byte, retained bool, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mqttPub == nil {
		return errors.New("mqtt publisher not started")
	}
	return c.mqttPub.Publish(topic, qos, retained, payload)
}

func (c *Connector) Server() *NatsServer              { return c.natsSvr }
func (c *Connector) AppConn() *nats.Conn              { return c.natsConn }
func (c *Connector) SysConn() *nats.Conn              { return c.sysConn }
func (c *Connector) JetStream() nats.JetStreamContext { return c.js }

func (c *Connector) OnLocalPresence(handler connector.PresenceHandler) {
	c.presenceHandler = handler
}

var _ connector.Connector = (*Connector)(nil)
