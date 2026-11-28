package embed

import (
	"bytes"
	"crypto/tls"

	"ruff.io/tio/connector"

	mqtt "github.com/wind-c/comqtt/v2/mqtt"
	"github.com/wind-c/comqtt/v2/mqtt/packets"
)

// principalExtKey stores the authenticated principal on the client for
// presence/forwarding, mirroring cl.Properties.Username.
const principalExtKey = "tio.principal"

// authHook authenticates connections and enforces the strict topic ACL.
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
	authCtx := connector.AuthContext{
		ClientIdentifier: pk.Connect.ClientIdentifier,
		Username:         string(pk.Connect.Username),
		Password:         string(pk.Connect.Password),
	}
	if tc, ok := cl.Net.Conn.(*tls.Conn); ok {
		if state := tc.ConnectionState(); len(state.PeerCertificates) > 0 {
			authCtx.HasClientCert = true
			authCtx.CertCN = state.PeerCertificates[0].Subject.CommonName
		}
	}

	result, ok := a.authzFn(authCtx)
	if !ok {
		return false
	}

	// The authenticated principal is the client's identity for ACL, presence
	// and forwarding, regardless of the CONNECT username.
	cl.Properties.Username = []byte(result.Principal)
	cl.Ext[principalExtKey] = result.Principal
	return true
}

func (a *authHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return a.aclFn(cl.ID, string(cl.Properties.Username), topic, write)
}
