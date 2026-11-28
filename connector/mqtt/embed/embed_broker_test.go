package embed

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("freePort close: %v", err)
	}
	return port
}

func allowAllAuthzFn(_ connector.AuthContext) (connector.AuthResult, bool) {
	return connector.AuthResult{}, true
}

func allowAllAclFn(clientId, username, topic string, write bool) bool {
	return true
}

func newTestBroker(t *testing.T) *Broker {
	t.Helper()
	return newTestBrokerWithStorage(t, config.InnerMqttStorage{})
}

func startBroker(t *testing.T, storage config.InnerMqttStorage) *Broker {
	t.Helper()
	b := InitBroker(Options{
		TcpPort: freePort(t),
		Storage: storage,
	})
	if err := b.ConfigureAuth(allowAllAuthzFn, allowAllAclFn); err != nil {
		t.Fatalf("ConfigureAuth: %v", err)
	}
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return b
}

func newTestBrokerWithStorage(t *testing.T, storage config.InnerMqttStorage) *Broker {
	t.Helper()
	b := startBroker(t, storage)
	t.Cleanup(func() {
		if b.svr != nil {
			_ = b.svr.Close()
		}
	})
	return b
}

func connectPaho(t *testing.T, port int, clientID string) mqtt.Client {
	t.Helper()
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", port)).
		SetClientID(clientID).
		SetUsername(clientID).
		SetCleanSession(true).
		SetAutoReconnect(false)
	c := mqtt.NewClient(opts)
	tok := c.Connect()
	if !tok.WaitTimeout(5 * time.Second) {
		t.Fatalf("connect %q: timeout", clientID)
	}
	if err := tok.Error(); err != nil {
		t.Fatalf("connect %q: %v", clientID, err)
	}
	t.Cleanup(func() { c.Disconnect(250) })
	return c
}

func subscribePaho(t *testing.T, c mqtt.Client, topic string) <-chan string {
	t.Helper()
	ch := make(chan string, 8)
	tok := c.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
		ch <- string(m.Payload())
	})
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("subscribe %q: %v", topic, tok.Error())
	}
	return ch
}

func waitConnected(t *testing.T, b *Broker, thingId string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if ok, err := b.IsConnected(thingId); err == nil && ok {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %q to connect", thingId)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func waitDisconnected(t *testing.T, c mqtt.Client) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for c.IsConnected() {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for client to disconnect")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func waitBrokerDisconnected(t *testing.T, b *Broker, thingId string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		// Poll the raw Clients map, not IsConnected: the client is removed only
		// after its OnDisconnect hook (storage delete) has completed, whereas
		// Closed() flips true earlier in processDisconnect.
		if _, ok := b.svr.Clients.Get(thingId); !ok {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for broker to drop %q", thingId)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestBrokerPublishSubscribeRoundTrip(t *testing.T) {
	b := newTestBroker(t)

	c := connectPaho(t, b.opt.TcpPort, "dev-rt")

	// broker -> client
	ch := subscribePaho(t, c, "t/x")
	if err := b.Publish("t/x", []byte("hi")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case got := <-ch:
		if got != "hi" {
			t.Fatalf("payload = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for broker Publish")
	}

	// client -> broker inline subscription
	received := make(chan connector.Message, 4)
	if err := b.Subscribe(context.Background(), "t/y", func(m connector.Message) {
		received <- m
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	pubTok := c.Publish("t/y", 0, false, "hello")
	if !pubTok.WaitTimeout(5*time.Second) || pubTok.Error() != nil {
		t.Fatalf("paho publish: %v", pubTok.Error())
	}
	select {
	case m := <-received:
		if m.Topic() != "t/y" || string(m.Payload()) != "hello" {
			t.Fatalf("topic=%q payload=%q", m.Topic(), m.Payload())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for inline Subscribe")
	}
}

func TestBrokerRetainedAndQoS(t *testing.T) {
	b := newTestBroker(t)

	// retained published before any subscriber
	if err := b.PublishRetained("t/retained", []byte("persisted")); err != nil {
		t.Fatalf("PublishRetained: %v", err)
	}

	c := connectPaho(t, b.opt.TcpPort, "dev-ret")
	chRet := subscribePaho(t, c, "t/retained")
	chRel := subscribePaho(t, c, "t/reliable")

	// retained delivered to a new subscriber
	select {
	case got := <-chRet:
		if got != "persisted" {
			t.Fatalf("retained payload = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for retained message")
	}

	// reliable publish delivered
	if err := b.PublishReliable("t/reliable", []byte("qos1")); err != nil {
		t.Fatalf("PublishReliable: %v", err)
	}
	select {
	case got := <-chRel:
		if got != "qos1" {
			t.Fatalf("reliable payload = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for reliable message")
	}
}

func TestBrokerConnectivity(t *testing.T) {
	b := newTestBroker(t)

	// unknown thing
	if ok, err := b.IsConnected("unknown"); err != nil || ok {
		t.Fatalf("IsConnected(unknown) = %v, %v", ok, err)
	}
	// ClientInfo cache is not populated until presence (Task 3)
	if _, err := b.ClientInfo("unknown"); err == nil {
		t.Fatal("expected ClientInfo error before presence cache is populated")
	}
	if infos, err := b.AllClientInfo(); err != nil || len(infos) != 0 {
		t.Fatalf("AllClientInfo = %v, %v", infos, err)
	}

	c := connectPaho(t, b.opt.TcpPort, "dev-conn")
	waitConnected(t, b, "dev-conn")

	if ok, err := b.IsConnected("dev-conn"); err != nil || !ok {
		t.Fatalf("IsConnected(dev-conn) = %v, %v", ok, err)
	}

	// Close disconnects the matching connection (matched by username)
	if err := b.Close("dev-conn"); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitDisconnected(t, c)

	if ok, _ := b.IsConnected("dev-conn"); ok {
		t.Fatal("expected disconnected after Close")
	}

	// Close with no match returns ErrConnectionNotFound
	if err := b.Close("dev-conn"); !errors.Is(err, connector.ErrConnectionNotFound) {
		t.Fatalf("Close(unknown) err = %v, want ErrConnectionNotFound", err)
	}

	// CloseAll is idempotent (no error when nothing matches)
	if err := b.CloseAll("dev-conn"); err != nil {
		t.Fatalf("CloseAll: %v", err)
	}
}

func TestBrokerFileStorageSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	storage := config.InnerMqttStorage{Type: "file", FilePath: dir}

	b1 := startBroker(t, storage)
	if err := b1.PublishRetained("persist/x", []byte("survived")); err != nil {
		t.Fatalf("PublishRetained: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // let badger flush the retained message
	if err := b1.svr.Close(); err != nil {
		t.Fatalf("close first server: %v", err)
	}

	b2 := newTestBrokerWithStorage(t, storage)
	c := connectPaho(t, b2.opt.TcpPort, "dev-persist")
	ch := subscribePaho(t, c, "persist/x")

	select {
	case got := <-ch:
		if got != "survived" {
			t.Fatalf("retained after restart = %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for retained message after restart")
	}

	// Disconnect cleanly and wait for the broker to finish its OnDisconnect
	// hook, so the cleanup's server Close does not race badger shutdown.
	c.Disconnect(250)
	waitDisconnected(t, c)
	waitBrokerDisconnected(t, b2, "dev-persist")
}
