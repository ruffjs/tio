package nats

import (
	"encoding/json"
	"testing"
	"time"

	server "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestIsConnectedFalseForUnknown(t *testing.T) {
	c := newPresenceTestConnector(t)

	connected, err := c.IsConnected("nonexistent-thing")
	if err != nil {
		t.Fatalf("IsConnected: %v", err)
	}
	if connected {
		t.Fatal("expected false for unknown thing")
	}
}

func TestIsConnectedTrueAfterConnect(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc := connectDevice(t, c, "conn-dev1")
	defer nc.Close()

	waitForPresence(t, c, "conn-dev1", true)

	connected, err := c.IsConnected("conn-dev1")
	if err != nil {
		t.Fatalf("IsConnected: %v", err)
	}
	if !connected {
		t.Fatal("expected true after connect")
	}
}

func TestAllClientInfoReturnsAll(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc1 := connectDevice(t, c, "all-dev1")
	nc2 := connectDevice(t, c, "all-dev2")
	defer nc1.Close()
	defer nc2.Close()

	waitForPresence(t, c, "all-dev1", true)
	waitForPresence(t, c, "all-dev2", true)

	infos, err := c.AllClientInfo()
	if err != nil {
		t.Fatalf("AllClientInfo: %v", err)
	}

	found := make(map[string]bool)
	for _, ci := range infos {
		found[ci.Username] = true
	}

	if !found["all-dev1"] || !found["all-dev2"] {
		t.Fatalf("expected all-dev1 and all-dev2, got %v", found)
	}
}

func TestCloseDisconnectsClient(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc, err := nats.Connect(
		c.natsSvr.ClientURL(),
		nats.UserInfo("close-dev", "test-password"),
		nats.Name("test-close-dev"),
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	waitForPresence(t, c, "close-dev", true)

	if err := c.Close("close-dev"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		if !nc.IsConnected() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for client disconnect after Close")
		case <-time.After(50 * time.Millisecond):
		}
	}

	waitForPresence(t, c, "close-dev", false)
}

func TestControlDisconnectConfirmsOwnerAction(t *testing.T) {
	c := newPresenceTestConnector(t)
	nc := connectDevice(t, c, "control-close-dev")
	waitForPresence(t, c, "control-close-dev", true)

	payload, _ := json.Marshal(controlDisconnectRequest{
		ThingId:  "control-close-dev",
		ServerId: c.cfg.Server.ServerName,
	})
	reply, err := c.natsConn.Request(controlDisconnectSubj, payload, time.Second)
	if err != nil {
		t.Fatalf("request control disconnect: %v", err)
	}
	var resp controlDisconnectResponse
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Disconnected {
		t.Fatalf("disconnect not confirmed: %+v", resp)
	}

	deadline := time.Now().Add(time.Second)
	for nc.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if nc.IsConnected() {
		t.Fatal("owner confirmed disconnect without closing the client")
	}
}

func TestCloseWaitsForRemoteConfirmation(t *testing.T) {
	c := newPresenceTestConnector(t)
	const thingId = "remote-close-dev"
	const remoteServer = "remote-node"

	rec, _ := json.Marshal(PresenceRecord{
		ThingId:   thingId,
		Connected: true,
		ServerId:  remoteServer,
	})
	if _, err := c.kv.Put(presenceKeyPrefix+thingId, rec); err != nil {
		t.Fatalf("put remote presence: %v", err)
	}

	_, err := c.natsConn.Subscribe(controlDisconnectSubj, func(msg *nats.Msg) {
		var req controlDisconnectRequest
		if json.Unmarshal(msg.Data, &req) == nil && req.ServerId == remoteServer && req.ThingId == thingId {
			data, _ := json.Marshal(controlDisconnectResponse{Disconnected: true})
			_ = msg.Respond(data)
		}
	})
	if err != nil {
		t.Fatalf("subscribe remote owner: %v", err)
	}
	if err := c.natsConn.Flush(); err != nil {
		t.Fatalf("flush remote owner: %v", err)
	}

	if err := c.Close(thingId); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRemoveClearsKVAndRetained(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc := connectDevice(t, c, "rm-dev")
	waitForPresence(t, c, "rm-dev", true)

	if err := c.Remove("rm-dev"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	nc.Close()

	time.Sleep(500 * time.Millisecond)

	_, err := c.kv.Get(presenceKeyPrefix + "rm-dev")
	if err != nats.ErrKeyNotFound {
		t.Fatalf("expected KeyNotFound after Remove, got err=%v", err)
	}
}

func TestClientInfoReturnsDetails(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc := connectDevice(t, c, "info-dev")
	defer nc.Close()

	waitForPresence(t, c, "info-dev", true)

	ci, err := c.ClientInfo("info-dev")
	if err != nil {
		t.Fatalf("ClientInfo: %v", err)
	}
	if ci.Username != "info-dev" {
		t.Fatalf("username = %q", ci.Username)
	}
	if !ci.Connected {
		t.Fatal("expected connected=true")
	}
	if ci.ConnectedAt == nil {
		t.Fatal("expected ConnectedAt to be set")
	}
}

func TestClientInfoNotFound(t *testing.T) {
	c := newPresenceTestConnector(t)

	_, err := c.ClientInfo("unknown-info-dev")
	if err == nil {
		t.Fatal("expected error for unknown thing")
	}
}

func TestReconcileFixesStaleEntries(t *testing.T) {
	c := newPresenceTestConnector(t)

	nc := connectDevice(t, c, "recon-dev")
	waitForPresence(t, c, "recon-dev", true)

	nc.Close()
	waitForPresence(t, c, "recon-dev", false)

	entry, _ := c.kv.Get(presenceKeyPrefix + "recon-dev")
	var rec PresenceRecord
	json.Unmarshal(entry.Value(), &rec)
	rec.Connected = true
	data, _ := json.Marshal(rec)
	c.kv.Put(presenceKeyPrefix+"recon-dev", data)

	time.Sleep(50 * time.Millisecond)
	c.reconcile()

	time.Sleep(100 * time.Millisecond)
	entry2, _ := c.kv.Get(presenceKeyPrefix + "recon-dev")
	var rec2 PresenceRecord
	json.Unmarshal(entry2.Value(), &rec2)
	if rec2.Connected {
		t.Fatal("reconcile should have fixed stale connected=true entry")
	}
}

func TestRequestAliveServersCollectsAllResponses(t *testing.T) {
	c := newPresenceTestConnector(t)
	const subject = "$tio.test.server.ping"

	for _, name := range []string{"node-a", "node-b"} {
		_, err := c.sysConn.Subscribe(subject, func(msg *nats.Msg) {
			data, _ := json.Marshal(server.ServerStatsMsg{
				Server: server.ServerInfo{Name: name},
			})
			_ = msg.Respond(data)
		})
		if err != nil {
			t.Fatalf("subscribe responder: %v", err)
		}
	}
	if err := c.sysConn.Flush(); err != nil {
		t.Fatalf("flush responders: %v", err)
	}

	alive := c.requestAliveServers(subject, time.Second)
	if !alive["node-a"] || !alive["node-b"] || len(alive) != 2 {
		t.Fatalf("expected both server responses, got %v", alive)
	}
}
