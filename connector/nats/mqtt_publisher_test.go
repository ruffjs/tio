package nats

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
)

type mqttTestAuth struct {
	mu     sync.Mutex
	appAcc *server.Account
}

func (a *mqttTestAuth) Check(c server.ClientAuthentication) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.appAcc == nil {
		return false
	}
	opts := c.GetOpts()
	c.RegisterUser(&server.User{
		Username: opts.Username,
		Password: opts.Password,
		Account:  a.appAcc,
	})
	return true
}

func getMqttPort(t *testing.T, s *NatsServer) int {
	t.Helper()
	v, err := s.Server().Varz(nil)
	require.NoError(t, err)
	if v.MQTT.Port <= 0 {
		t.Fatal("MQTT port not assigned")
	}
	return v.MQTT.Port
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func testMqttSubscriber(t *testing.T, port int, user, password string) mqtt.Client {
	t.Helper()
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://127.0.0.1:%d", port)).
		SetClientID(fmt.Sprintf("test-sub-%s-%d", t.Name(), time.Now().UnixNano())).
		SetUsername(user).
		SetPassword(password).
		SetAutoReconnect(false)
	c := mqtt.NewClient(opts)
	token := c.Connect()
	token.Wait()
	if token.Error() != nil {
		t.Fatalf("subscriber connect: %v", token.Error())
	}
	t.Cleanup(func() { c.Disconnect(100) })
	return c
}

func startMqttTestServer(t *testing.T, mqttPort int) (*NatsServer, *mqttTestAuth) {
	t.Helper()
	cfg := testServerConfig(t)
	cfg.MqttPort = mqttPort
	auth := &mqttTestAuth{}
	s, err := StartNatsServer(cfg, auth)
	require.NoError(t, err)
	t.Cleanup(func() {
		if s.IsRunning() {
			s.Shutdown()
		}
	})

	appAcc, err := s.Server().LookupAccount(AppAccountName)
	require.NoError(t, err)
	auth.mu.Lock()
	auth.appAcc = appAcc
	auth.mu.Unlock()

	return s, auth
}

func TestMQTTPublisher_Connect(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("test-server", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	pub.mu.Lock()
	require.True(t, pub.connected)
	require.NotNil(t, pub.client)
	pub.mu.Unlock()
}

func TestMQTTPublisher_ConnectContextCancel(t *testing.T) {
	pub := newMqttPublisher("test-server", 19999, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := pub.Connect(ctx)
	require.Error(t, err)
}

func TestMQTTPublisher_QoS1Delivery(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("qos1-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	sub := testMqttSubscriber(t, mqttPort, "sub-user", "sub-pass")

	received := make(chan mqtt.Message, 1)
	topic := "devices/test/values"
	subToken := sub.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
		received <- m
	})
	subToken.Wait()
	require.NoError(t, subToken.Error())

	err = pub.Publish(topic, 1, false, []byte("hello-qos1"))
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, "hello-qos1", string(msg.Payload()))
		require.Equal(t, topic, msg.Topic())
		require.Equal(t, byte(1), msg.Qos())
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for QoS 1 message")
	}
}

func TestMQTTPublisher_QoS0Delivery(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("qos0-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	sub := testMqttSubscriber(t, mqttPort, "sub-user", "sub-pass")

	received := make(chan mqtt.Message, 1)
	topic := "devices/test/qos0"
	subToken := sub.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
		received <- m
	})
	subToken.Wait()
	require.NoError(t, subToken.Error())

	err = pub.Publish(topic, 0, false, []byte("hello-qos0"))
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, "hello-qos0", string(msg.Payload()))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for QoS 0 message")
	}
}

func TestMQTTPublisher_RetainedMessage(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("retain-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	topic := "devices/retain/test"
	err = pub.Publish(topic, 1, true, []byte("retained-value"))
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	sub := testMqttSubscriber(t, mqttPort, "sub-user", "sub-pass")

	received := make(chan mqtt.Message, 1)
	subToken := sub.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
		received <- m
	})
	subToken.Wait()
	require.NoError(t, subToken.Error())

	select {
	case msg := <-received:
		require.Equal(t, "retained-value", string(msg.Payload()))
		require.True(t, msg.Retained())
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for retained message")
	}
}

func TestMQTTPublisher_ClearRetained(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("clear-retain-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	topic := "devices/clear-retain/test"

	err = pub.Publish(topic, 1, true, []byte("retained-value"))
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	err = pub.Publish(topic, 1, true, []byte{})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	sub := testMqttSubscriber(t, mqttPort, "sub-user", "sub-pass")

	received := make(chan mqtt.Message, 1)
	subToken := sub.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
		received <- m
	})
	subToken.Wait()
	require.NoError(t, subToken.Error())

	select {
	case <-received:
		t.Fatal("should NOT receive any retained message after clearing")
	case <-time.After(1 * time.Second):
	}
}

func TestMQTTPublisher_PublishAfterDisconnect(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("disc-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)

	pub.Disconnect()

	err = pub.Publish("devices/test/values", 1, false, []byte("should-fail"))
	require.Error(t, err)
}

func TestMQTTPublisher_InvalidTopic(t *testing.T) {
	s, _ := startMqttTestServer(t, -1)
	mqttPort := getMqttPort(t, s)

	pub := newMqttPublisher("topic-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	err = pub.Publish("", 1, false, []byte("data"))
	require.Error(t, err)

	err = pub.Publish("has spaces/topic", 1, false, []byte("data"))
	require.Error(t, err)

	err = pub.Publish("has+/wildcard", 1, false, []byte("data"))
	require.Error(t, err)
}

func TestMQTTPublisher_Reconnect(t *testing.T) {
	mqttPort := freeTCPPort(t)
	time.Sleep(100 * time.Millisecond)

	s1, _ := startMqttTestServer(t, mqttPort)
	_ = s1

	pub := newMqttPublisher("reconnect-test", mqttPort, "pub-user", "pub-pass")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pub.Connect(ctx)
	require.NoError(t, err)
	defer pub.Disconnect()

	s1.Shutdown()

	time.Sleep(200 * time.Millisecond)

	s2, _ := startMqttTestServer(t, mqttPort)
	_ = s2

	require.Eventually(t, func() bool {
		pub.mu.Lock()
		defer pub.mu.Unlock()
		return pub.client != nil && pub.client.IsConnected()
	}, 10*time.Second, 200*time.Millisecond, "publisher did not auto-reconnect")

	sub := testMqttSubscriber(t, mqttPort, "sub-user", "sub-pass")
	received := make(chan mqtt.Message, 1)
	topic := "devices/reconnect/values"
	subToken := sub.Subscribe(topic, 1, func(c mqtt.Client, m mqtt.Message) {
		received <- m
	})
	subToken.Wait()
	require.NoError(t, subToken.Error())

	err = pub.Publish(topic, 1, false, []byte("after-reconnect"))
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, "after-reconnect", string(msg.Payload()))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message after reconnect")
	}
}
