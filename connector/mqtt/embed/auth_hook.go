package embed

import (
	"bytes"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

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
	return a.authzFn(ConnectParams(pk.Connect))
}

func (a *authHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return a.aclFn(string(cl.Properties.Username), topic, write)
}
