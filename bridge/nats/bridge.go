package nats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ruff.io/tio/config"

	"github.com/nats-io/nats.go"
)

// JetStreamOptions configures the optional JetStream persistence of the
// egress bridge. When Enabled, the bridge ensures a TIO_MQTT stream exists.
type JetStreamOptions struct {
	Enabled       bool
	StoreDir      string
	MaxMemory     int64
	MaxStore      int64
	MaxMsgs       int64
	MaxBytes      int64
	MaxAgeSeconds int64
}

// Config configures the optional NATS bridge.
type Config struct {
	Enabled      bool
	Mode         string // "embedded" | "external"
	URL          string // external mode server URL
	Server       config.NatsServerConfig
	AppClient    config.NatsClientConfig
	SystemClient config.NatsClientConfig
	JetStream    JetStreamOptions
}

const streamName = "TIO_MQTT"

// Bridge is the optional outbound NATS bridge. It implements the
// connector/mqtt/embed.Bridge interface (Forward/ForwardPresence).
type Bridge struct {
	cfg        Config
	superUsers []config.UserPassword

	appConn *nats.Conn
	srv     *NatsServer
	js      nats.JetStreamContext
}

func New(cfg Config, superUsers []config.UserPassword) (*Bridge, error) {
	return &Bridge{cfg: cfg, superUsers: superUsers}, nil
}

func (b *Bridge) Enabled() bool { return b.cfg.Enabled }

func (b *Bridge) Start(ctx context.Context) error {
	if !b.Enabled() {
		return nil
	}

	switch b.cfg.Mode {
	case "", "embedded":
		authn := NewNatsAuthenticator(b.superUsers, b.cfg.AppClient, b.cfg.SystemClient)
		srv, err := StartNatsServer(b.cfg.Server, b.cfg.JetStream, authn)
		if err != nil {
			return err
		}
		appAcc, err := srv.Server().LookupAccount(AppAccountName)
		if err != nil {
			srv.Shutdown()
			return fmt.Errorf("lookup APP account: %w", err)
		}
		sysAcc, err := srv.Server().LookupAccount(SysAccountName)
		if err != nil {
			srv.Shutdown()
			return fmt.Errorf("lookup SYS account: %w", err)
		}
		authn.SetAccounts(appAcc, sysAcc)

		nc, err := nats.Connect(srv.ClientURL(),
			nats.UserInfo(b.cfg.AppClient.User, b.cfg.AppClient.Password),
			nats.Name("tio-nats-bridge"))
		if err != nil {
			srv.Shutdown()
			return fmt.Errorf("connect embedded nats: %w", err)
		}
		b.appConn = nc
		b.srv = srv
		srv.logStartup()
	case "external":
		nc, err := nats.Connect(b.cfg.URL,
			nats.UserInfo(b.cfg.AppClient.User, b.cfg.AppClient.Password),
			nats.Name("tio-nats-bridge"))
		if err != nil {
			return fmt.Errorf("connect external nats: %w", err)
		}
		b.appConn = nc
	default:
		return fmt.Errorf("unknown nats mode %q", b.cfg.Mode)
	}

	if b.cfg.JetStream.Enabled {
		js, err := b.appConn.JetStream()
		if err != nil {
			b.cleanup()
			return fmt.Errorf("jetstream context: %w", err)
		}
		b.js = js
		if err := b.ensureStream(); err != nil {
			b.cleanup()
			return err
		}
	}

	return nil
}

func (b *Bridge) cleanup() {
	if b.appConn != nil {
		b.appConn.Close()
		b.appConn = nil
	}
	if b.srv != nil {
		b.srv.Shutdown()
		b.srv = nil
	}
}

func (b *Bridge) ensureStream() error {
	cfg := b.cfg.JetStream
	sc := &nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"$iothub.>", "tio.>"},
		MaxMsgs:  cfg.MaxMsgs,
		MaxBytes: cfg.MaxBytes,
	}
	if cfg.MaxAgeSeconds > 0 {
		sc.MaxAge = time.Duration(cfg.MaxAgeSeconds) * time.Second
	}

	_, err := b.js.AddStream(sc)
	if err == nil {
		return nil
	}
	if !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return fmt.Errorf("add stream %s: %w", streamName, err)
	}
	if _, err := b.js.UpdateStream(sc); err != nil {
		return fmt.Errorf("update stream %s: %w", streamName, err)
	}
	return nil
}

func (b *Bridge) Shutdown() error {
	b.cleanup()
	b.js = nil
	return nil
}

func (b *Bridge) Server() *NatsServer { return b.srv }

func (b *Bridge) ClientURL() string {
	if b.srv != nil {
		return b.srv.ClientURL()
	}
	return ""
}

func (b *Bridge) AppConn() *nats.Conn { return b.appConn }

func (b *Bridge) JetStream() nats.JetStreamContext { return b.js }
