package integration_tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"
)

// TestWebSocketMQTTConnection tests MQTT over WebSocket connection
func TestWebSocketMQTTConnection(t *testing.T) {
	// Get the WebSocket port from the NATS connector
	wsPort := natsConnector.Server().WsPort()
	if wsPort == 0 {
		t.Skip("WebSocket port not configured, skipping test")
	}

	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create WebSocket MQTT client
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/mqtt", wsPort)
	opts := mqtt.NewClientOptions().
		AddBroker(wsURL).
		SetClientID(thingId).
		SetUsername(thingId).
		SetPassword("test-password").
		SetAutoReconnect(false).
		SetCleanSession(true).
		SetConnectTimeout(5 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	
	select {
	case <-ctx.Done():
		t.Fatal("connection timeout")
	case <-token.Done():
		if token.Error() != nil {
			t.Fatalf("WebSocket MQTT connection failed: %v", token.Error())
		}
	}

	require.True(t, client.IsConnected(), "client should be connected")

	// Test publish and subscribe
	received := make(chan bool, 1)
	testTopic := fmt.Sprintf("$iothub/things/%s/shadow/update", thingId)
	
	subToken := client.Subscribe(testTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		received <- true
	})
	subToken.Wait()
	require.NoError(t, subToken.Error(), "subscribe should succeed")

	// Publish a message
	pubToken := client.Publish(testTopic, 0, false, []byte(`{"test":"data"}`))
	pubToken.Wait()
	require.NoError(t, pubToken.Error(), "publish should succeed")

	// Wait for message
	select {
	case <-received:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive published message")
	}

	client.Disconnect(250)
}

// TestWebSocketMQTTAuth tests that invalid credentials are rejected over WebSocket
func TestWebSocketMQTTAuth(t *testing.T) {
	wsPort := natsConnector.Server().WsPort()
	if wsPort == 0 {
		t.Skip("WebSocket port not configured, skipping test")
	}

	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to connect with wrong password
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/mqtt", wsPort)
	opts := mqtt.NewClientOptions().
		AddBroker(wsURL).
		SetClientID(thingId).
		SetUsername(thingId).
		SetPassword("wrong-password").
		SetAutoReconnect(false).
		SetCleanSession(true).
		SetConnectTimeout(3 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	
	select {
	case <-ctx.Done():
		// Timeout is acceptable for rejected connection
	case <-token.Done():
		if token.Error() == nil {
			client.Disconnect(250)
			t.Fatal("connection with wrong password should be rejected")
		}
		// Connection rejected as expected
	}
}
