package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/require"

	tioconnector "ruff.io/tio/connector"
	mq "ruff.io/tio/internal/mqtttest"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/shadow"
)

func newSuperClient(ctx context.Context, name string) *mq.DeviceClient {
	port := natsConnector.Server().MqttPort()
	return mq.NewDeviceClient(
		fmt.Sprintf("tcp://127.0.0.1:%d", port),
		name,
		cfg.Connector.Nats.SuperUsers[0].Name,
		cfg.Connector.Nats.SuperUsers[0].Password,
	)
}

func TestPasswordAuth(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err, "device with valid password should connect")
	client.Disconnect()
}

func TestInvalidPasswordRejected(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "wrong-password")
	err := client.Connect(ctx)
	require.Error(t, err, "device with wrong password should be rejected")
}

func TestCrossThingAccessDenied(t *testing.T) {
	thingA := ID()
	thingB := ID()
	crateThing(thingA)
	crateThing(thingB)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientA := newThingMqttClient(ctx, thingA, "test-password")
	err := clientA.Connect(ctx)
	require.NoError(t, err)
	defer clientA.Disconnect()

	received := int32(0)
	err = clientA.Subscribe(shadow.TopicDeltaStateOf(thingB), 0, func(_ mqtt.Client, m mqtt.Message) {
		atomic.AddInt32(&received, 1)
	})
	require.NoError(t, err)

	payload, _ := json.Marshal(shadow.DeltaStateNotice{
		Version: 1,
		State:   shadow.StateValue{"on": true},
	})
	require.NoError(t, natsConnector.PublishReliable(shadow.TopicDeltaStateOf(thingB), payload))

	time.Sleep(500 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&received), "thing A should not receive thing B's messages")
}

func TestQoS1Delivery(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	received := make(chan []byte, 1)
	topic := "$iothub/things/" + thingId + "/test/delivery"
	err = client.Subscribe(topic, 1, func(_ mqtt.Client, m mqtt.Message) {
		received <- m.Payload()
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	payload := []byte(`{"hello":"qos1"}`)
	require.NoError(t, natsConnector.PublishReliable(topic, payload))

	select {
	case msg := <-received:
		require.Equal(t, payload, msg)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for QoS 1 message")
	}
}

func TestRetainedDelivery(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "$iothub/things/" + thingId + "/test/retained"
	payload := []byte(`{"retained":true}`)
	require.NoError(t, natsConnector.PublishRetained(topic, payload))

	time.Sleep(200 * time.Millisecond)

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	received := make(chan []byte, 1)
	err = client.Subscribe(topic, 1, func(_ mqtt.Client, m mqtt.Message) {
		received <- m.Payload()
	})
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, payload, msg)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retained message")
	}
}

func TestRetainedClear(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic := "$iothub/things/" + thingId + "/test/retained-clear"
	payload := []byte(`{"retained":true}`)
	require.NoError(t, natsConnector.PublishRetained(topic, payload))
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, natsConnector.PublishRetained(topic, []byte{}))
	time.Sleep(200 * time.Millisecond)

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	received := int32(0)
	err = client.Subscribe(topic, 1, func(_ mqtt.Client, m mqtt.Message) {
		if len(m.Payload()) > 0 {
			atomic.AddInt32(&received, 1)
		}
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&received), "retained message should have been cleared")
}

func TestBroadcastSubscribe(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	obsA := ID()
	obsB := ID()
	crateThing(obsA)
	crateThing(obsB)

	clientA := newSuperClient(ctx, obsA+"-obs")
	err := clientA.Connect(ctx)
	require.NoError(t, err)
	defer clientA.Disconnect()

	clientB := newSuperClient(ctx, obsB+"-obs")
	err = clientB.Connect(ctx)
	require.NoError(t, err)
	defer clientB.Disconnect()

	topic := "$iothub/things/" + thingId + "/test/broadcast"

	var receivedA, receivedB int32
	err = clientA.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
		atomic.AddInt32(&receivedA, 1)
	})
	require.NoError(t, err)
	err = clientB.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
		atomic.AddInt32(&receivedB, 1)
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	payload := []byte(`{"broadcast":true}`)
	require.NoError(t, natsConnector.PublishReliable(topic, payload))

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&receivedA) >= 1
	}, 5*time.Second, 50*time.Millisecond, "subscriber A should receive broadcast")
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&receivedB) >= 1
	}, 5*time.Second, 50*time.Millisecond, "subscriber B should receive broadcast")
}

func TestQueueSubscribe(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	topic := "$iothub/things/" + thingId + "/test/queue"

	received := int32(0)
	err = client.Subscribe(topic, 0, func(_ mqtt.Client, m mqtt.Message) {
		atomic.AddInt32(&received, 1)
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"msg":%d}`, i))
		require.NoError(t, natsConnector.PublishReliable(topic, payload))
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&received) >= 5
	}, 5*time.Second, 50*time.Millisecond, "all messages should be received")
}

func TestShadowSetDesiredAcceptance(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	deltaReceived := make(chan struct{}, 1)
	err = client.Subscribe(shadow.TopicDeltaStateOf(thingId), 0, func(_ mqtt.Client, m mqtt.Message) {
		var notice shadow.DeltaStateNotice
		err := json.Unmarshal(m.Payload(), &notice)
		if err == nil && notice.State["color"] == "blue" {
			select {
			case deltaReceived <- struct{}{}:
			default:
			}
		}
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	body := strings.NewReader(`{
		"clientToken": "test-token-desired",
		"state": {
			"desired": {
				"color": "blue"
			}
		}
	}`)
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId), body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody rest.Resp[any]
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respBody.Code)

	select {
	case <-deltaReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for delta state notification")
	}
}

func TestShadowSetReportedAcceptance(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	acceptedReceived := make(chan struct{}, 1)
	err = client.Subscribe(shadow.TopicUpdateAcceptedOf(thingId), 1, func(_ mqtt.Client, m mqtt.Message) {
		var resp shadow.StateAcceptedResp
		err := json.Unmarshal(m.Payload(), &resp)
		if err == nil && resp.ClientToken == "test-token-reported" {
			select {
			case acceptedReceived <- struct{}{}:
			default:
			}
		}
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	stateReq := shadow.StateReq{
		ClientToken: "test-token-reported",
		State: shadow.StateDR{
			Reported: shadow.StateValue{"temperature": 25},
		},
	}
	stateReqBytes, _ := json.Marshal(stateReq)
	err = client.Publish(shadow.TopicUpdateOf(thingId), 1, false, stateReqBytes)
	require.NoError(t, err)

	select {
	case <-acceptedReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for accepted notification")
	}
}

func TestPresenceConnectDisconnect(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	observerId := ID()
	crateThing(observerId)
	observer := newSuperClient(ctx, observerId+"-obs")
	err := observer.Connect(ctx)
	require.NoError(t, err)
	defer observer.Disconnect()

	presenceTopic := tioconnector.TopicPresenceEvent(thingId)
	connectReceived := make(chan struct{}, 1)
	disconnectReceived := make(chan struct{}, 1)

	err = observer.Subscribe(presenceTopic, 1, func(_ mqtt.Client, m mqtt.Message) {
		var evt tioconnector.PresenceEvent
		if err := json.Unmarshal(m.Payload(), &evt); err != nil {
			return
		}
		if evt.EventType == tioconnector.EventConnected {
			select {
			case connectReceived <- struct{}{}:
			default:
			}
		} else if evt.EventType == tioconnector.EventDisconnected {
			select {
			case disconnectReceived <- struct{}{}:
			default:
			}
		}
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	device := newThingMqttClient(ctx, thingId, "test-password")
	err = device.Connect(ctx)
	require.NoError(t, err)

	select {
	case <-connectReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for connect presence event")
	}

	device.Disconnect()

	select {
	case <-disconnectReceived:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for disconnect presence event")
	}
}

func TestMultiplePresenceSubscribers(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	obsA := ID()
	obsB := ID()
	crateThing(obsA)
	crateThing(obsB)

	clientA := newSuperClient(ctx, obsA+"-obsA")
	err := clientA.Connect(ctx)
	require.NoError(t, err)
	defer clientA.Disconnect()

	clientB := newSuperClient(ctx, obsB+"-obsB")
	err = clientB.Connect(ctx)
	require.NoError(t, err)
	defer clientB.Disconnect()

	presenceTopic := tioconnector.TopicPresenceEvent(thingId)
	var receivedA, receivedB int32

	err = clientA.Subscribe(presenceTopic, 1, func(_ mqtt.Client, m mqtt.Message) {
		atomic.AddInt32(&receivedA, 1)
	})
	require.NoError(t, err)
	err = clientB.Subscribe(presenceTopic, 1, func(_ mqtt.Client, m mqtt.Message) {
		atomic.AddInt32(&receivedB, 1)
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	device := newThingMqttClient(ctx, thingId, "test-password")
	err = device.Connect(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&receivedA) >= 1
	}, 5*time.Second, 50*time.Millisecond, "subscriber A should receive presence event")
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&receivedB) >= 1
	}, 5*time.Second, 50*time.Millisecond, "subscriber B should receive presence event")

	device.Disconnect()
}

func TestWildcardSubscription(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := newThingMqttClient(ctx, thingId, "test-password")
	err := client.Connect(ctx)
	require.NoError(t, err)
	defer client.Disconnect()

	var mu sync.Mutex
	receivedTopics := make(map[string]bool)

	err = client.Subscribe("$iothub/things/"+thingId+"/wildcard-test/#", 0, func(_ mqtt.Client, m mqtt.Message) {
		mu.Lock()
		receivedTopics[m.Topic()] = true
		mu.Unlock()
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	payload := []byte(`{"test":true}`)
	require.NoError(t, natsConnector.Publish("$iothub/things/"+thingId+"/wildcard-test", payload))
	require.NoError(t, natsConnector.Publish("$iothub/things/"+thingId+"/wildcard-test/sub", payload))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return receivedTopics["$iothub/things/"+thingId+"/wildcard-test"] &&
			receivedTopics["$iothub/things/"+thingId+"/wildcard-test/sub"]
	}, 5*time.Second, 50*time.Millisecond, "wildcard subscription should receive both exact and sub-level messages")
}

func TestInvalidPublishTopic(t *testing.T) {
	err := natsConnector.Publish("foo/#/bar", []byte("test"))
	require.Error(t, err, "publishing with wildcard # should return error")

	err = natsConnector.Publish("foo/+/bar", []byte("test"))
	require.Error(t, err, "publishing with wildcard + should return error")
}

func TestJobApiAbsent(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, httpSvr.URL+"/api/v1/jobs", nil)
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "Job endpoint should return 404")
}
