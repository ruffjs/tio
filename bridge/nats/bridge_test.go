package nats

import (
	"context"
	"testing"
	"time"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestBridgeEmbeddedForward(t *testing.T) {
	cfg := testBridgeConfig(t)
	b, err := New(cfg, nil)
	require.NoError(t, err)
	require.NoError(t, b.Start(context.Background()))
	defer b.Shutdown()

	require.NotNil(t, b.Server())
	require.NotEmpty(t, b.ClientURL())

	nc, err := nats.Connect(b.ClientURL(),
		nats.UserInfo(cfg.AppClient.User, cfg.AppClient.Password),
		nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	sub, err := nc.SubscribeSync("tio.>")
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	b.Forward("tio/dev1/data", []byte("up-data"))
	b.ForwardPresence(connector.PresenceEvent{ThingId: "dev1", EventType: connector.EventConnected}, []byte("presence-payload"))

	got := make(map[string][]byte, 2)
	deadline := time.After(3 * time.Second)
	for len(got) < 2 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for messages, got %v", got)
		default:
		}
		msg, err := sub.NextMsg(100 * time.Millisecond)
		if err != nil {
			continue
		}
		got[msg.Subject] = msg.Data
	}

	require.Equal(t, []byte("up-data"), got["tio.dev1.data"])
	require.Equal(t, []byte("presence-payload"), got["tio.dev1.presence"])
}

func TestBridgeDisabled(t *testing.T) {
	b, err := New(Config{Enabled: false}, nil)
	require.NoError(t, err)
	require.False(t, b.Enabled())

	require.NoError(t, b.Start(context.Background()))
	require.Nil(t, b.Server())
	require.Empty(t, b.ClientURL())
	require.Nil(t, b.AppConn())
	require.Nil(t, b.JetStream())

	// Forward/ForwardPresence must be no-ops, not panics.
	b.Forward("tio/dev1/data", []byte("x"))
	b.ForwardPresence(connector.PresenceEvent{ThingId: "dev1"}, []byte("y"))

	require.NoError(t, b.Shutdown())
}

func TestBridgeEmbeddedJetStream(t *testing.T) {
	cfg := testBridgeConfig(t)
	cfg.JetStream = JetStreamOptions{
		Enabled:       true,
		StoreDir:      t.TempDir(),
		MaxAgeSeconds: 60,
		MaxMsgs:       100,
		MaxBytes:      1024,
	}

	b, err := New(cfg, nil)
	require.NoError(t, err)
	require.NoError(t, b.Start(context.Background()))
	defer b.Shutdown()

	require.NotNil(t, b.JetStream())

	info, err := b.JetStream().StreamInfo(streamName)
	require.NoError(t, err)
	require.Equal(t, []string{"$iothub.>", "tio.>"}, info.Config.Subjects)
	require.Equal(t, 60*time.Second, info.Config.MaxAge)
	require.Equal(t, int64(100), info.Config.MaxMsgs)
	require.Equal(t, int64(1024), info.Config.MaxBytes)
}

func testBridgeConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Enabled: true,
		Mode:    "embedded",
		Server: config.NatsServerConfig{
			ServerName:  "test-" + t.Name(),
			ClusterName: "test-cluster",
			Port:        -1,
			MonitorPort: 0,
		},
		AppClient:    config.NatsClientConfig{User: "app-user", Password: "app-pass"},
		SystemClient: config.NatsClientConfig{User: "sys-user", Password: "sys-pass"},
	}
}
