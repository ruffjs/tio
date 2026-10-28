package nats

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ruff.io/tio/connector"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func usernameAuthzFn(ctx connector.AuthContext) (connector.AuthResult, bool) {
	return connector.AuthResult{Principal: ctx.Username, AuthMethod: "test"}, true
}

func newPresenceTestConnector(t *testing.T) *Connector {
	t.Helper()
	cfg := testConnectorConfig(t)
	c, err := NewConnector(cfg, testDeviceCodec(t), "legacy")
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	if err := c.ConfigureAuth(usernameAuthzFn, nil); err != nil {
		t.Fatalf("ConfigureAuth: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = c.Shutdown() })
	return c
}

func connectDevice(t *testing.T, c *Connector, thingId string) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect(
		c.natsSvr.ClientURL(),
		nats.UserInfo(thingId, "test-password"),
		nats.Name("test-device-"+thingId),
	)
	if err != nil {
		t.Fatalf("connect device %q: %v", thingId, err)
	}
	t.Cleanup(func() { nc.Close() })
	return nc
}

func waitForPresence(t *testing.T, c *Connector, thingId string, connected bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		isConn, err := c.IsConnected(thingId)
		if err == nil && isConn == connected {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for presence thingId=%q connected=%v", thingId, connected)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestPresenceConnectEvent(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc := connectDevice(t, c, "dev1")
	defer nc.Close()

	waitForPresence(t, c, "dev1", true)

	entry, err := c.kv.Get(presenceKeyPrefix + "dev1")
	if err != nil {
		t.Fatalf("get KV: %v", err)
	}
	var rec PresenceRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !rec.Connected {
		t.Fatal("expected connected=true")
	}
	if rec.ThingId != "dev1" {
		t.Fatalf("thingId = %q", rec.ThingId)
	}
}

func TestPresenceDisconnectEvent(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc := connectDevice(t, c, "dev2")
	waitForPresence(t, c, "dev2", true)

	nc.Close()

	waitForPresence(t, c, "dev2", false)

	entry, err := c.kv.Get(presenceKeyPrefix + "dev2")
	if err != nil {
		t.Fatalf("get KV: %v", err)
	}
	var rec PresenceRecord
	if err := json.Unmarshal(entry.Value(), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rec.Connected {
		t.Fatal("expected connected=false after disconnect")
	}
}

func TestPresenceOnLocalPresenceCallback(t *testing.T) {
	c := newPresenceTestConnector(t)

	var received []connector.ClientInfo
	var mu sync.Mutex
	done := make(chan struct{})

	c.OnLocalPresence(func(ci connector.ClientInfo) {
		mu.Lock()
		received = append(received, ci)
		mu.Unlock()
		if ci.Connected && ci.Username == "dev3" {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})

	nc := connectDevice(t, c, "dev3")
	defer nc.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for OnLocalPresence callback")
	}

	mu.Lock()
	found := false
	for _, ci := range received {
		if ci.Username == "dev3" && ci.Connected {
			found = true
			break
		}
	}
	mu.Unlock()
	if !found {
		t.Fatal("expected connected callback for dev3")
	}
}

func TestPresenceReconnectUpdatesRecord(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc1 := connectDevice(t, c, "dev4")
	waitForPresence(t, c, "dev4", true)

	nc1.Close()
	waitForPresence(t, c, "dev4", false)

	nc2 := connectDevice(t, c, "dev4")
	defer nc2.Close()
	waitForPresence(t, c, "dev4", true)

	entry2, _ := c.kv.Get(presenceKeyPrefix + "dev4")
	var rec2 PresenceRecord
	json.Unmarshal(entry2.Value(), &rec2)
	if !rec2.Connected {
		t.Fatal("expected connected=true after reconnect")
	}
}

func TestPresenceInternalUsersIgnored(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc, err := nats.Connect(
		c.natsSvr.ClientURL(),
		nats.UserInfo(c.cfg.AppClient.User, c.cfg.AppClient.Password),
		nats.Name("internal-app"),
	)
	if err != nil {
		t.Fatalf("connect internal: %v", err)
	}
	defer nc.Close()

	time.Sleep(500 * time.Millisecond)

	infos, err := c.AllClientInfo()
	if err != nil {
		t.Fatalf("AllClientInfo: %v", err)
	}
	for _, ci := range infos {
		if ci.Username == c.cfg.AppClient.User {
			t.Fatal("internal user should not be in presence records")
		}
	}
}

// Hold reconnect attempts while devices change state on the running broker.
type presenceDialer struct {
	calls   atomic.Int32
	waiting chan struct{}
	resume  chan struct{}
}

func (d *presenceDialer) Dial(network, address string) (net.Conn, error) {
	if d.calls.Add(1) > 1 {
		select {
		case d.waiting <- struct{}{}:
		default:
		}
		select {
		case <-d.resume:
		default:
			return nil, net.ErrClosed
		}
	}
	return net.DialTimeout(network, address, time.Second)
}

func pausePresenceConnection(t *testing.T, c *Connector) func() {
	t.Helper()
	d := &presenceDialer{waiting: make(chan struct{}, 1), resume: make(chan struct{})}
	callback := c.sysConn.Opts.ReconnectedCB
	c.sysConn.Close()
	nc, err := nats.Connect(c.natsSvr.ClientURL(),
		nats.UserInfo(c.cfg.SystemClient.User, c.cfg.SystemClient.Password),
		nats.SetCustomDialer(d), nats.ReconnectWait(10*time.Millisecond),
		nats.ReconnectHandler(callback))
	require.NoError(t, err)
	c.mu.Lock()
	c.sysConn = nc
	c.mu.Unlock()
	_, err = nc.Subscribe(sysConnectSubj, c.handleConnectEvent)
	require.NoError(t, err)
	_, err = nc.Subscribe(sysDisconnectSubj, c.handleDisconnectEvent)
	require.NoError(t, err)
	require.NoError(t, nc.FlushTimeout(time.Second))
	require.NoError(t, nc.ForceReconnect())
	select {
	case <-d.waiting:
	case <-time.After(2 * time.Second):
		t.Fatal("system connection did not attempt reconnect")
	}
	require.Eventually(t, func() bool {
		page, err := c.natsSvr.Server().Connz(&server.ConnzOptions{Username: true, Account: SysAccountName})
		return err == nil && len(page.Conns) == 0
	}, time.Second, 10*time.Millisecond)
	return func() { close(d.resume) }
}

func TestPresenceCompensatesSystemReconnect(t *testing.T) {
	c := newPresenceTestConnector(t)
	old := testMqttSubscriber(t, getMqttPort(t, c.natsSvr), "old-device", "password")
	waitForPresence(t, c, "old-device", true)
	changes := make(chan connector.ClientInfo, 10)
	c.OnLocalPresence(func(ci connector.ClientInfo) { changes <- ci })

	remote := PresenceRecord{ThingId: "remote-device", ServerId: "other-node", Connected: true}
	data, err := json.Marshal(remote)
	require.NoError(t, err)
	_, err = c.kv.Put(presenceKeyPrefix+remote.ThingId, data)
	require.NoError(t, err)

	resume := pausePresenceConnection(t, c)
	old.Disconnect(100)
	testMqttSubscriber(t, getMqttPort(t, c.natsSvr), "new-device", "password")
	connected, err := c.IsConnected("new-device")
	require.NoError(t, err)
	require.False(t, connected)
	connected, err = c.IsConnected("old-device")
	require.NoError(t, err)
	require.True(t, connected)
	resume()

	waitForPresence(t, c, "new-device", true)
	waitForPresence(t, c, "old-device", false)
	for i := 0; i < 2; i++ {
		select {
		case ci := <-changes:
			require.Equal(t, ci.Username == "new-device", ci.Connected)
		case <-time.After(2 * time.Second):
			t.Fatal("missing compensated local presence callback")
		}
	}
	entry, err := c.kv.Get(presenceKeyPrefix + remote.ThingId)
	require.NoError(t, err)
	require.JSONEq(t, string(data), string(entry.Value()))
	c.compensatePresence()
	select {
	case ci := <-changes:
		t.Fatalf("duplicate compensated presence: %+v", ci)
	default:
	}
}

func TestPresenceQueuedDisconnectAfterReconnect(t *testing.T) {
	c := newPresenceTestConnector(t)
	nc := connectDevice(t, c, "reconnected-device")
	waitForPresence(t, c, "reconnected-device", true)
	entry, err := c.kv.Get(presenceKeyPrefix + "reconnected-device")
	require.NoError(t, err)
	data, err := json.Marshal(sysDisconnectEvent{
		Server: sysServerInfo{Name: c.cfg.Server.ServerName},
		Client: sysClientInfo{User: "reconnected-device"},
	})
	require.NoError(t, err)
	c.handleDisconnectEvent(&nats.Msg{Data: data})
	after, err := c.kv.Get(presenceKeyPrefix + "reconnected-device")
	require.NoError(t, err)
	require.Equal(t, entry.Revision(), after.Revision())
	nc.Close()
	waitForPresence(t, c, "reconnected-device", false)
	data, err = json.Marshal(sysConnectEvent{
		Server: sysServerInfo{Name: c.cfg.Server.ServerName},
		Client: sysClientInfo{User: "reconnected-device"},
	})
	require.NoError(t, err)
	c.handleConnectEvent(&nats.Msg{Data: data})
	connected, err := c.IsConnected("reconnected-device")
	require.NoError(t, err)
	require.False(t, connected)
}

func TestPresenceCompensationAfterShutdown(t *testing.T) {
	c := newPresenceTestConnector(t)
	resume := pausePresenceConnection(t, c)
	if err := c.Shutdown(); err != nil {
		require.ErrorIs(t, err, nats.ErrConnectionReconnecting)
	}
	resume()
	c.compensatePresence()
}

func TestPresenceScanAllConnections(t *testing.T) {
	c := newPresenceTestConnector(t)
	for i := 0; i < server.DefaultConnListSize; i++ {
		nc, err := nats.Connect(c.natsSvr.ClientURL(),
			nats.UserInfo(c.cfg.AppClient.User, c.cfg.AppClient.Password), nats.NoReconnect())
		require.NoError(t, err)
		defer nc.Close()
	}
	connectDevice(t, c, "last-device")
	c.presenceMu.Lock()
	conns, err := c.localConnections("")
	c.presenceMu.Unlock()
	require.NoError(t, err)
	require.Greater(t, len(conns), server.DefaultConnListSize)
	found := false
	for _, ci := range conns {
		if ci.AuthorizedUser == "last-device" {
			found = true
		}
	}
	require.True(t, found, "connection after the first page must be scanned")

	c.mu.Lock()
	c.presenceMu.Lock()
	original := c.ctx
	ctx, cancel := context.WithCancel(original)
	defer cancel()
	c.ctx = &singlePageScanContext{Context: ctx, cancel: cancel}
	filtered, err := c.localConnections("last-device")
	c.ctx = original
	c.presenceMu.Unlock()
	c.mu.Unlock()
	require.NoError(t, err, "a short filtered page must finish without checking for another page")
	require.Len(t, filtered, 1)
}

// Interleave a competing node's write with the real JetStream CAS operation.
type interleavedPresenceKV struct {
	nats.KeyValue
	beforeWrite func(string)
	writeErr    error
	writes      int
}

func (kv *interleavedPresenceKV) Create(key string, value []byte) (uint64, error) {
	kv.writes++
	if kv.beforeWrite != nil {
		kv.beforeWrite(key)
	}
	if kv.writeErr != nil {
		return 0, kv.writeErr
	}
	return kv.KeyValue.Create(key, value)
}

func (kv *interleavedPresenceKV) Update(key string, value []byte, revision uint64) (uint64, error) {
	kv.writes++
	if kv.beforeWrite != nil {
		kv.beforeWrite(key)
	}
	if kv.writeErr != nil {
		return 0, kv.writeErr
	}
	return kv.KeyValue.Update(key, value, revision)
}

func TestPresenceRetriesRevisionConflicts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		conflicts int
		create    bool
	}{
		{name: "update", conflicts: 1},
		{name: "create", conflicts: 1, create: true},
		{name: "deferred", conflicts: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newPresenceTestConnector(t)
			connectDevice(t, c, "moving-device")
			waitForPresence(t, c, "moving-device", true)
			c.presenceMu.Lock()
			key := presenceKeyPrefix + "moving-device"
			base := c.kv
			t.Cleanup(func() {
				c.presenceMu.Lock()
				defer c.presenceMu.Unlock()
				c.presenceHandler = nil
				c.kv = base
			})
			remote := PresenceRecord{ThingId: "moving-device", ServerId: "other-node", Connected: false}
			data, err := json.Marshal(remote)
			require.NoError(t, err)
			if tc.create {
				require.NoError(t, base.Delete(key))
			} else {
				_, err = base.Put(key, data)
				require.NoError(t, err)
			}
			kv := &interleavedPresenceKV{KeyValue: base}
			kv.beforeWrite = func(key string) {
				if kv.writes <= tc.conflicts {
					_, err := base.Put(key, data)
					require.NoError(t, err)
				}
			}
			c.kv = kv
			changes := 0
			c.presenceHandler = func(ci connector.ClientInfo) {
				require.True(t, ci.Connected)
				changes++
			}
			c.syncLocalPresence("moving-device", "")
			writes, notifications := kv.writes, changes
			c.presenceMu.Unlock()
			if tc.conflicts == 3 {
				require.Equal(t, 3, writes)
				require.Zero(t, notifications)
				c.reconcile()
			} else {
				require.Equal(t, 2, writes)
			}
			c.presenceMu.Lock()
			defer c.presenceMu.Unlock()
			entry, err := base.Get(key)
			require.NoError(t, err)
			var rec PresenceRecord
			require.NoError(t, json.Unmarshal(entry.Value(), &rec))
			require.True(t, rec.Connected)
			require.Equal(t, c.cfg.Server.ServerName, rec.ServerId)
			require.Equal(t, 1, changes)
			c.syncLocalPresence("moving-device", "")
			require.Equal(t, 1, changes)
		})
	}
}

func TestPresenceConflictRechecksConnectionsAndOwner(t *testing.T) {
	c := newPresenceTestConnector(t)
	nc := connectDevice(t, c, "moving-device")
	waitForPresence(t, c, "moving-device", true)
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	base := c.kv
	t.Cleanup(func() {
		c.presenceMu.Lock()
		defer c.presenceMu.Unlock()
		c.presenceHandler = nil
		c.kv = base
	})
	key := presenceKeyPrefix + "moving-device"
	_, err := base.Put(key, []byte(`{"thingId":"moving-device","connected":false}`))
	require.NoError(t, err)
	kv := &interleavedPresenceKV{KeyValue: base}
	kv.beforeWrite = func(key string) {
		nc.Close()
		require.Eventually(t, func() bool {
			page, err := c.natsSvr.Server().Connz(&server.ConnzOptions{Username: true, User: "moving-device"})
			return err == nil && len(page.Conns) == 0
		}, time.Second, time.Millisecond)
		_, err := base.Put(key, []byte(`{"thingId":"moving-device","serverId":"other-node","connected":true}`))
		require.NoError(t, err)
	}
	c.kv = kv
	c.presenceHandler = func(connector.ClientInfo) { t.Error("stale presence notification") }
	c.syncLocalPresence("moving-device", "")
	entry, err := base.Get(key)
	require.NoError(t, err)
	var rec PresenceRecord
	require.NoError(t, json.Unmarshal(entry.Value(), &rec))
	require.Equal(t, "other-node", rec.ServerId)
	require.True(t, rec.Connected)
	require.Equal(t, 1, kv.writes)
}

// Cancel at the next page boundary to detect an unnecessary continuation.
type singlePageScanContext struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *singlePageScanContext) Err() error {
	c.checks++
	if c.checks > 1 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestPresenceDoesNotRetryOtherWriteErrors(t *testing.T) {
	c := newPresenceTestConnector(t)
	connectDevice(t, c, "moving-device")
	waitForPresence(t, c, "moving-device", true)
	c.presenceMu.Lock()
	defer c.presenceMu.Unlock()
	base := c.kv
	defer func() { c.kv = base; c.presenceHandler = nil }()
	_, err := base.Put(presenceKeyPrefix+"moving-device", []byte(`{"thingId":"moving-device","connected":false}`))
	require.NoError(t, err)
	kv := &interleavedPresenceKV{KeyValue: base, writeErr: nats.ErrTimeout}
	c.kv = kv
	c.presenceHandler = func(connector.ClientInfo) { t.Error("notification for failed write") }
	c.syncLocalPresence("moving-device", "")
	require.Equal(t, 1, kv.writes)
	require.Empty(t, c.pendingPresence)
}
