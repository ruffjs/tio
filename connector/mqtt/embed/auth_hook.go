package embed

import (
	"bytes"

	"ruff.io/tio/connector"

	mqtt "github.com/wind-c/comqtt/v2/mqtt"
	"github.com/wind-c/comqtt/v2/mqtt/packets"
)

// authHook is a pass-through authentication hook. Task 2 adds principal
// extraction and the rewritten ACL matrix.
type authHook struct {
	mqtt.HookBase
	authzFn AuthzFn
	aclFn   AclFn
}

func (a *authHook) ID() string {
	return "auth"
}

func (a *authHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

func (a *authHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	_, ok := a.authzFn(connector.AuthContext{
		ClientIdentifier: pk.Connect.ClientIdentifier,
		Username:         string(pk.Connect.Username),
		Password:         string(pk.Connect.Password),
	})
	return ok
}

func (a *authHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return a.aclFn(cl.ID, string(cl.Properties.Username), topic, write)
}
