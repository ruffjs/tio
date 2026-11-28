package embed

import (
	"context"
	"testing"
	"time"

	"ruff.io/tio/connector"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestForwardable(t *testing.T) {
	tests := []struct {
		mode  string
		topic string
		want  bool
	}{
		// legacy forwarded
		{"legacy", "$iothub/user/things/dev1/x", true},
		{"legacy", "$iothub/user/things/dev1/deep/path", true},
		{"legacy", "$iothub/things/dev1/property", true},
		{"legacy", "$iothub/things/dev1/messages/property", true},
		{"legacy", "$iothub/things/dev1/ota/task", true},
		// legacy control-plane denied
		{"legacy", "$iothub/things/dev1/shadows/name/default/update", false},
		{"legacy", "$iothub/things/dev1/methods/m/resp", false},
		{"legacy", "$iothub/things/dev1/ntp/req", false},
		{"legacy", "$iothub/things/dev1/ntp/resp", false},
		{"legacy", "$iothub/things/dev1/jobs", false},
		{"legacy", "$iothub/things/dev1/jobs/notify", false},
		{"legacy", "$iothub/things/dev1/presence", false},
		{"legacy", "$iothub/things/dev1/will", false},
		{"legacy", "$iothub/things/dev1/will/x", false},
		{"legacy", "$iothub/things/dev1/messages/will", false},
		{"legacy", "$iothub/things/dev1/messages/will/x", false},
		// legacy rejects simple topics
		{"legacy", "tio/dev1/event", false},
		{"legacy", "tio/dev1/data", false},

		// simple forwarded
		{"simple", "tio/dev1/event", true},
		{"simple", "tio/dev1/event/sub", true},
		{"simple", "tio/dev1/data", true},
		{"simple", "tio/dev1/data/sub", true},
		{"simple", "tio/dev1/custom", true},
		{"simple", "tio/dev1/custom/ota/task", true},
		{"simple", "tio/dev1/up/ota_status", true},
		{"simple", "tio/dev1/up/custom_thing", true},
		// simple control-plane / downlink denied
		{"simple", "tio/dev1/up/shadow_get", false},
		{"simple", "tio/dev1/up/shadow_update", false},
		{"simple", "tio/dev1/up/method_resp", false},
		{"simple", "tio/dev1/up/ntp_req", false},
		{"simple", "tio/dev1/down/x", false},
		{"simple", "tio/dev1/unknown_dir/x", false},
		// simple rejects legacy topics
		{"simple", "$iothub/user/things/dev1/x", false},
		{"simple", "$iothub/things/dev1/property", false},

		// dual forwards both protocol families
		{"dual", "$iothub/user/things/dev1/x", true},
		{"dual", "$iothub/things/dev1/property", true},
		{"dual", "$iothub/things/dev1/ota/task", true},
		{"dual", "tio/dev1/event", true},
		{"dual", "tio/dev1/data/sub", true},
		{"dual", "tio/dev1/custom/ota/task", true},
		{"dual", "tio/dev1/up/ota_status", true},
		// dual denies control-plane on both sides
		{"dual", "$iothub/things/dev1/shadows/name/default/update", false},
		{"dual", "$iothub/things/dev1/ntp/req", false},
		{"dual", "$iothub/things/dev1/presence", false},
		{"dual", "$iothub/things/dev1/will/x", false},
		{"dual", "$iothub/things/dev1/messages/will/x", false},
		{"dual", "tio/dev1/up/shadow_get", false},
		{"dual", "tio/dev1/up/shadow_update", false},
		{"dual", "tio/dev1/up/method_resp", false},
		{"dual", "tio/dev1/up/ntp_req", false},
		{"dual", "tio/dev1/down/x", false},

		// always denied across modes
		{"legacy", "$SYS/x", false},
		{"simple", "$SYS/x", false},
		{"dual", "$SYS/x", false},
		{"legacy", "$tio/control", false},
		{"simple", "$tio/control", false},
		{"dual", "$tio/control", false},
		{"legacy", "other/x", false},
		{"simple", "other/x", false},
		{"dual", "other/x", false},

		// degenerate topics without a sub-path are not forwarded
		{"legacy", "$iothub/things/dev1", false},
		{"legacy", "$iothub/user/things/dev1", false},
		{"legacy", "$iothub/events/things/dev1/presence", false},

		// unknown/empty mode never forwards
		{"bogus", "$iothub/user/things/dev1/x", false},
		{"", "tio/dev1/event", false},
	}

	for _, tt := range tests {
		if got := forwardable(tt.mode, tt.topic); got != tt.want {
			t.Errorf("forwardable(%q, %q) = %v, want %v", tt.mode, tt.topic, got, tt.want)
		}
	}
}

type fwd struct {
	topic   string
	payload []byte
}

type recordingBridge struct {
	fwd chan fwd
}

func (rb *recordingBridge) Forward(topic string, payload []byte) {
	rb.fwd <- fwd{topic: topic, payload: payload}
}

func (rb *recordingBridge) ForwardPresence(evt connector.PresenceEvent, payload []byte) {}

func publishAndWait(t *testing.T, c mqtt.Client, topic, payload string) {
	t.Helper()
	tok := c.Publish(topic, 1, false, payload)
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("publish %q: %v", topic, tok.Error())
	}
}

func wantForward(t *testing.T, rb *recordingBridge, topic, payload string) {
	t.Helper()
	select {
	case f := <-rb.fwd:
		if f.topic != topic || string(f.payload) != payload {
			t.Fatalf("forward = %q/%q, want %q/%q", f.topic, f.payload, topic, payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for forward of %q", topic)
	}
}

func assertNoForward(t *testing.T, rb *recordingBridge) {
	t.Helper()
	select {
	case f := <-rb.fwd:
		t.Fatalf("unexpected forward: %q/%q", f.topic, f.payload)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBrokerForwardHook(t *testing.T) {
	rb := &recordingBridge{fwd: make(chan fwd, 16)}
	b := InitBroker(Options{TcpPort: freePort(t), ProtocolMode: "dual", Bridge: rb})
	if err := b.ConfigureAuth(allowAllAuthzFn, allowAllAclFn); err != nil {
		t.Fatalf("ConfigureAuth: %v", err)
	}
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.svr.Close() })

	dev := connectPaho(t, b.opt.TcpPort, "dev-fwd")
	waitConnected(t, b, "dev-fwd")

	// device publishes whitelisted simple and legacy topics -> forwarded
	publishAndWait(t, dev, "tio/dev-fwd/event", "up-event")
	wantForward(t, rb, "tio/dev-fwd/event", "up-event")
	publishAndWait(t, dev, "$iothub/user/things/dev-fwd/x", "up-legacy")
	wantForward(t, rb, "$iothub/user/things/dev-fwd/x", "up-legacy")

	// device publishes control-plane topics -> not forwarded
	publishAndWait(t, dev, "tio/dev-fwd/up/shadow_get", "{}")
	publishAndWait(t, dev, "$iothub/things/dev-fwd/shadows/name/default/update", "{}")
	assertNoForward(t, rb)

	// inline (server) publish -> not forwarded even for a whitelisted topic
	if err := b.Publish("tio/dev-fwd/data", []byte("inline")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	assertNoForward(t, rb)
}
