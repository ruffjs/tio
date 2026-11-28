//go:build integration

package integration_tests

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
)

// bridgeAppConn returns the bridge's NATS connection so tests can observe what
// external applications receive.
func bridgeAppConn(t *testing.T) *nats.Conn {
	t.Helper()
	require.True(t, natsBrg.Enabled(), "NATS bridge must be enabled for integration tests")
	nc := natsBrg.AppConn()
	require.NotNil(t, nc, "NATS bridge is not started")
	return nc
}

type natsMsg struct {
	subject string
	data    []byte
}

type natsRecorder struct {
	mu   sync.Mutex
	msgs []natsMsg
}

func newNatsRecorder(t *testing.T, nc *nats.Conn, subjects ...string) *natsRecorder {
	t.Helper()
	r := &natsRecorder{}
	for _, subject := range subjects {
		_, err := nc.Subscribe(subject, func(m *nats.Msg) {
			r.mu.Lock()
			r.msgs = append(r.msgs, natsMsg{subject: m.Subject, data: append([]byte(nil), m.Data...)})
			r.mu.Unlock()
		})
		require.NoError(t, err, "subscribe %q", subject)
	}
	require.NoError(t, nc.Flush())
	return r
}

func (r *natsRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.msgs)
}

func (r *natsRecorder) all() []natsMsg {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]natsMsg(nil), r.msgs...)
}

func TestDeviceUplinkForwardedToNats(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	device := newThingMqttClient(ctx, thingId, "test-password")
	require.NoError(t, device.Connect(ctx))
	defer device.Disconnect()
	waitConnected(t, thingId)

	legacySubject := "$iothub.user.things." + thingId + ".x"
	dataSubject := "tio." + thingId + ".data"
	otaSubject := "tio." + thingId + ".up.ota_status"
	r := newNatsRecorder(t, bridgeAppConn(t), legacySubject, dataSubject, otaSubject)

	type uplink struct{ topic, subject, payload string }
	var ups []uplink
	if cfg.Protocol.Mode == "legacy" || cfg.Protocol.Mode == "dual" {
		ups = append(ups, uplink{
			topic:   "$iothub/user/things/" + thingId + "/x",
			subject: legacySubject,
			payload: "legacy-uplink",
		})
	}
	if cfg.Protocol.Mode == "simple" || cfg.Protocol.Mode == "dual" {
		ups = append(ups,
			uplink{topic: "tio/" + thingId + "/data", subject: dataSubject, payload: "simple-data"},
			uplink{topic: "tio/" + thingId + "/up/ota_status", subject: otaSubject, payload: "simple-ota"},
		)
	}
	require.NotEmpty(t, ups, "no uplink topics apply to protocol mode %q", cfg.Protocol.Mode)

	for _, up := range ups {
		require.NoError(t, device.Publish(up.topic, 1, false, up.payload), "publish %q", up.topic)
	}

	require.Eventually(t, func() bool {
		got := map[string]string{}
		for _, m := range r.all() {
			got[m.subject] = string(m.data)
		}
		for _, up := range ups {
			if got[up.subject] != up.payload {
				return false
			}
		}
		return true
	}, 5*time.Second, 20*time.Millisecond, "uplinks were not forwarded to NATS")

	got := map[string]string{}
	for _, m := range r.all() {
		got[m.subject] = string(m.data)
	}
	for _, up := range ups {
		require.Equal(t, up.payload, got[up.subject], "subject %q", up.subject)
	}
}

func TestControlPlaneNotForwardedToNats(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	device := newThingMqttClient(ctx, thingId, "test-password")
	require.NoError(t, device.Connect(ctx))
	defer device.Disconnect()
	waitConnected(t, thingId)

	presenceSubject := "tio." + thingId + ".presence"
	r := newNatsRecorder(t, bridgeAppConn(t),
		"$iothub.things."+thingId+".>",
		"tio."+thingId+".>",
	)

	// Presence is the one forward allowed on tio.{id}.>; the delta downlink is
	// awaited below so the negative assertion covers a message the server really
	// produced.
	delta := make(chan struct{}, 1)
	deltaTopic := shadow.TopicDeltaStateOf(thingId)
	if cfg.Protocol.Mode == "simple" {
		deltaTopic = protocol.TopicDown(thingId, protocol.TypeShadowDesired)
	}
	require.NoError(t, device.Subscribe(deltaTopic, 1, func(_ mqtt.Client, _ mqtt.Message) {
		select {
		case delta <- struct{}{}:
		default:
		}
	}))

	// Device control-plane publishes: shadow updates are never forwarded.
	switch cfg.Protocol.Mode {
	case "legacy":
		require.NoError(t, device.Publish("$iothub/things/"+thingId+"/shadows/name/default/update", 1, false, "{}"))
	case "simple":
		require.NoError(t, device.Publish(protocol.TopicUp(thingId, protocol.TypeShadowUpdate), 1, false, "{}"))
	case "dual":
		require.NoError(t, device.Publish("$iothub/things/"+thingId+"/shadows/name/default/update", 1, false, "{}"))
		require.NoError(t, device.Publish(protocol.TopicUp(thingId, protocol.TypeShadowUpdate), 1, false, "{}"))
	}

	// A server-side shadow update produces a $iothub/tio downlink; that server
	// publish must not be forwarded either.
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/v1/things/%s/shadows/default/state/desired", httpSvr.URL, thingId),
		strings.NewReader(`{"clientToken":"e2e-nats","state":{"desired":{"e2e":"v"}}}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpSvr.Client().Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case <-delta:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the shadow delta downlink")
	}

	// Positive control: a forwardable device publish on the same wildcard family
	// proves the recorder subscription and the bridge forward path are live, so
	// the absence check below is meaningful.
	controlTopic, controlSubject := "$iothub/things/"+thingId+"/property", "$iothub.things."+thingId+".property"
	if cfg.Protocol.Mode == "simple" {
		controlTopic, controlSubject = "tio/"+thingId+"/data", "tio."+thingId+".data"
	}
	require.NoError(t, device.Publish(controlTopic, 1, false, "forward-control"))
	require.Eventually(t, func() bool {
		for _, m := range r.all() {
			if m.subject == controlSubject && string(m.data) == "forward-control" {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond, "positive control was not forwarded to NATS")

	var unexpected []string
	require.Never(t, func() bool {
		unexpected = unexpected[:0]
		for _, m := range r.all() {
			// presence is the one allowed forward; controlSubject is the
			// positive control that proves the channel is live.
			if m.subject == presenceSubject || m.subject == controlSubject {
				continue
			}
			unexpected = append(unexpected, m.subject)
		}
		return len(unexpected) > 0
	}, 300*time.Millisecond, 10*time.Millisecond, func() string {
		return fmt.Sprintf("control-plane messages must not be forwarded to NATS, got %v", unexpected)
	})
}

func TestPresenceForwardedOnce(t *testing.T) {
	thingId := ID()
	crateThing(thingId)

	r := newNatsRecorder(t, bridgeAppConn(t), "tio."+thingId+".presence")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	device := newThingMqttClient(ctx, thingId, "test-password")
	require.NoError(t, device.Connect(ctx))
	defer device.Disconnect()
	waitConnected(t, thingId)
	require.Eventually(t, func() bool { return r.count() >= 1 }, 5*time.Second, 10*time.Millisecond,
		"connect presence was not forwarded to NATS")

	device.Disconnect()
	waitDisconnected(t, thingId)
	require.Eventually(t, func() bool { return r.count() >= 2 }, 5*time.Second, 10*time.Millisecond,
		"disconnect presence was not forwarded to NATS")

	// Presence is forwarded once: no duplicate connected/disconnected events.
	require.Never(t, func() bool { return r.count() > 2 }, 500*time.Millisecond, 10*time.Millisecond,
		"presence forwarded more than once")

	msgs := r.all()
	require.Len(t, msgs, 2)

	var eventTypes []string
	for _, m := range msgs {
		var evt connector.PresenceEvent
		require.NoError(t, testCodec.Unmarshal(m.data, &evt), "presence payload %q", m.data)
		require.Equal(t, thingId, evt.ThingId)
		require.Greater(t, evt.TimestampUs, int64(0), "presence timestampUs")
		eventTypes = append(eventTypes, evt.EventType)
	}
	require.ElementsMatch(t,
		[]string{connector.EventConnected, connector.EventDisconnected}, eventTypes)
}
