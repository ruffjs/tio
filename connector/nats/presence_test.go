package nats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ruff.io/tio/connector"

	"github.com/nats-io/nats.go"
)

func usernameAuthzFn(ctx connector.AuthContext) (connector.AuthResult, bool) {
	return connector.AuthResult{Principal: ctx.Username, AuthMethod: "test"}, true
}

func newPresenceTestConnector(t *testing.T) *Connector {
	t.Helper()
	cfg := testConnectorConfig(t)
	c, err := NewConnector(cfg)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	if err := c.ConfigureAuth(usernameAuthzFn, allowAllAclFn); err != nil {
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
	if rec.Generation != 1 {
		t.Fatalf("generation = %d, want 1", rec.Generation)
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

func TestPresenceMultipleSubscribersReceiveEvent(t *testing.T) {
	c := newPresenceTestConnector(t)

	ctx := context.Background()
	ch1 := c.SubscribePresence(ctx)
	ch2 := c.SubscribePresence(ctx)
	ch3 := c.SubscribePresence(ctx)

	nc := connectDevice(t, c, "dev3")
	defer nc.Close()

	waitForPresence(t, c, "dev3", true)

	for i, ch := range []<-chan connector.PresenceEvent{ch1, ch2, ch3} {
		select {
		case evt := <-ch:
			if evt.ThingId != "dev3" {
				t.Fatalf("subscriber %d: thingId = %q", i, evt.ThingId)
			}
			if evt.EventType != connector.EventConnected {
				t.Fatalf("subscriber %d: eventType = %q", i, evt.EventType)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("subscriber %d: timeout waiting for event", i)
		}
	}
}

func TestPresenceGenerationTracking(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc1 := connectDevice(t, c, "dev4")
	waitForPresence(t, c, "dev4", true)

	entry1, _ := c.kv.Get(presenceKeyPrefix + "dev4")
	var rec1 PresenceRecord
	json.Unmarshal(entry1.Value(), &rec1)
	if rec1.Generation != 1 {
		t.Fatalf("first gen = %d, want 1", rec1.Generation)
	}

	nc1.Close()
	waitForPresence(t, c, "dev4", false)

	nc2 := connectDevice(t, c, "dev4")
	defer nc2.Close()
	waitForPresence(t, c, "dev4", true)

	entry2, _ := c.kv.Get(presenceKeyPrefix + "dev4")
	var rec2 PresenceRecord
	json.Unmarshal(entry2.Value(), &rec2)
	if rec2.Generation != 2 {
		t.Fatalf("second gen = %d, want 2", rec2.Generation)
	}
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
