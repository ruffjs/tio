package nats

import (
	"context"
	"sync"
	"testing"
	"time"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
)

func testConnectorConfig(t *testing.T) config.NatsConfig {
	t.Helper()
	return config.NatsConfig{
		Server:        testServerConfig(t),
		AppClient:     config.NatsClientConfig{User: "$tio-app", Password: "app-secret"},
		SystemClient:  config.NatsClientConfig{User: "$tio-sys", Password: "sys-secret"},
		MqttPublisher: config.NatsClientConfig{User: "$tio-mqtt-publisher", Password: "mqtt-secret"},
	}
}

func allowAllAuthzFn(_ connector.AuthContext) (connector.AuthResult, bool) {
	return connector.AuthResult{Principal: "test-thing", AuthMethod: "allow-all"}, true
}

func testDeviceCodec(t *testing.T) codec.Codec {
	t.Helper()
	c, err := codec.New("json")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newTestConnector(t *testing.T) *Connector {
	t.Helper()
	cfg := testConnectorConfig(t)
	c, err := NewConnector(cfg, testDeviceCodec(t), "legacy")
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	if err := c.ConfigureAuth(allowAllAuthzFn, nil); err != nil {
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

func TestConnectorFullLifecycle(t *testing.T) {
	c := newTestConnector(t)
	if c.Server() == nil || c.AppConn() == nil || c.SysConn() == nil || c.JetStream() == nil {
		t.Fatal("accessors returned nil after start")
	}
}

func TestStartBeforeConfigureFails(t *testing.T) {
	cfg := testConnectorConfig(t)
	c, err := NewConnector(cfg, testDeviceCodec(t), "legacy")
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail without ConfigureAuth")
	}
}

func TestConfigureAuthTwiceFails(t *testing.T) {
	cfg := testConnectorConfig(t)
	c, err := NewConnector(cfg, testDeviceCodec(t), "legacy")
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	if err := c.ConfigureAuth(allowAllAuthzFn, nil); err != nil {
		t.Fatalf("first ConfigureAuth: %v", err)
	}
	if err := c.ConfigureAuth(allowAllAuthzFn, nil); err == nil {
		t.Fatal("expected second ConfigureAuth to fail")
	}
}

func TestPublishNatsCore(t *testing.T) {
	c := newTestConnector(t)

	received := make(chan connector.Message, 1)
	ctx := context.Background()
	if err := c.Subscribe(ctx, "$iothub/things/dev1/data", func(msg connector.Message) { received <- msg }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := c.Publish("$iothub/things/dev1/data", []byte("hello-nats")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case msg := <-received:
		if msg.Topic() != "$iothub/things/dev1/data" || string(msg.Payload()) != "hello-nats" {
			t.Fatalf("got topic=%q payload=%q", msg.Topic(), msg.Payload())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestPublishReliableAndRetained(t *testing.T) {
	c := newTestConnector(t)

	reliableReceived := make(chan connector.Message, 1)
	retainedReceived := make(chan connector.Message, 1)

	ctx := context.Background()
	_ = c.Subscribe(ctx, "$iothub/things/dev1/rel", func(msg connector.Message) { reliableReceived <- msg })
	_ = c.Subscribe(ctx, "$iothub/things/dev1/ret", func(msg connector.Message) { retainedReceived <- msg })
	time.Sleep(50 * time.Millisecond)

	if err := c.PublishReliable("$iothub/things/dev1/rel", []byte("qos1")); err != nil {
		t.Fatalf("PublishReliable: %v", err)
	}
	if err := c.PublishRetained("$iothub/things/dev1/ret", []byte("retained")); err != nil {
		t.Fatalf("PublishRetained: %v", err)
	}

	var gotRel, gotRet bool
	timeout := time.After(5 * time.Second)
	for !gotRel || !gotRet {
		select {
		case msg := <-reliableReceived:
			if string(msg.Payload()) != "qos1" {
				t.Fatalf("reliable payload = %q", msg.Payload())
			}
			gotRel = true
		case msg := <-retainedReceived:
			if string(msg.Payload()) != "retained" {
				t.Fatalf("retained payload = %q", msg.Payload())
			}
			gotRet = true
		case <-timeout:
			t.Fatalf("timeout: reliable=%v retained=%v", gotRel, gotRet)
		}
	}
}

func TestConnectorCleanShutdown(t *testing.T) {
	cfg := testConnectorConfig(t)
	c, err := NewConnector(cfg, testDeviceCodec(t), "legacy")
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	if err := c.ConfigureAuth(allowAllAuthzFn, nil); err != nil {
		t.Fatalf("ConfigureAuth: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- c.Shutdown() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timed out")
	}

	if c.AppConn() != nil && c.AppConn().IsConnected() {
		t.Fatal("app connection should be closed")
	}
}

func TestPublishRejectsWildcardTopic(t *testing.T) {
	c := newTestConnector(t)
	if err := c.Publish("$iothub/things/+/data", []byte("x")); err == nil {
		t.Fatal("expected Publish to reject wildcard topic")
	}
}

func TestConcurrentPublishSubscribe(t *testing.T) {
	c := newTestConnector(t)

	const n = 20
	var wg sync.WaitGroup
	received := make(chan struct{}, n)

	ctx := context.Background()
	_ = c.Subscribe(ctx, "$iothub/things/dev1/concurrent/#", func(msg connector.Message) { received <- struct{}{} })
	time.Sleep(50 * time.Millisecond)

	for range n {
		wg.Go(func() {
			_ = c.Publish("$iothub/things/dev1/concurrent/item", []byte("data"))
		})
	}
	wg.Wait()

	got := 0
	timeout := time.After(3 * time.Second)
loop:
	for got < n {
		select {
		case <-received:
			got++
		case <-timeout:
			break loop
		}
	}
	if got == 0 {
		t.Fatal("no messages received")
	}
}
