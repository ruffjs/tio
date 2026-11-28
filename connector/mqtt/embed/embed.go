package embed

// embedded comqtt broker: tio's single MQTT entry point.

import (
	"sync/atomic"

	"ruff.io/tio/auth"
	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
)

// AuthzFn authenticates an incoming MQTT connection and returns the
// authenticated principal. See connector.AuthzFn.
type AuthzFn = connector.AuthzFn

// AclFn decides whether a client may read/write a topic. See auth.AclFn.
type AclFn = auth.AclFn

// Bridge is the optional outbound bridge (NATS). Inline publishes (server
// downlink, presence) are forwarded by the publish hook in a later task.
type Bridge interface {
	Forward(topic string, payload []byte)
	ForwardPresence(evt connector.PresenceEvent, payload []byte)
}

// Options configures the embedded comqtt broker.
type Options struct {
	TcpPort           int
	TcpSslPort        int
	WsPort            int
	WssPort           int
	CertFile          string
	KeyFile           string
	ClientCAFile      string
	RequireClientCert bool
	Storage           config.InnerMqttStorage
	Codec             codec.Codec
	ProtocolMode      string
	Bridge            Bridge
}

var brokerInstance atomic.Pointer[Broker]

// BrokerInstance returns the Broker created by the most recent InitBroker call.
func BrokerInstance() *Broker {
	return brokerInstance.Load()
}

// InitBroker creates a new Broker and records it as the package singleton.
// Each call returns a fresh instance (no sync.Once) so tests can create many.
func InitBroker(opt Options) *Broker {
	b := &Broker{opt: opt, codec: opt.Codec, bridge: opt.Bridge}
	brokerInstance.Store(b)
	return b
}
