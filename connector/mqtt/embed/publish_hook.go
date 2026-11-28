package embed

import (
	"bytes"

	mqtt "github.com/wind-c/comqtt/v2/mqtt"
	"github.com/wind-c/comqtt/v2/mqtt/packets"
)

// publishHook forwards whitelisted device publishes to the bridge (NATS).
// Inline publishes (server downlink, presence) are never forwarded.
type publishHook struct {
	mqtt.HookBase
	b *Broker
}

func (h *publishHook) ID() string {
	return "publish"
}

func (h *publishHook) Provides(b byte) bool {
	return bytes.Contains([]byte{mqtt.OnPublish}, []byte{b})
}

func (h *publishHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	if h.b.bridge != nil && !cl.Net.Inline && forwardable(h.b.opt.ProtocolMode, pk.TopicName) {
		h.b.bridge.Forward(pk.TopicName, pk.Payload)
	}
	return pk, nil
}
