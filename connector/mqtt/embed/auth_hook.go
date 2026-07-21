package embed

import (
	"bytes"
	"crypto/tls"
	"log/slog"
	"net"
	"reflect"

	"ruff.io/tio/connector"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

type authHook struct {
	mqtt.HookBase
	authzFn connector.AuthzFn
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

func extractCNFromCert(cl *mqtt.Client) (string, bool) {
	if cl == nil || cl.Net.Conn == nil {
		return "", false
	}

	tlsConn, ok := unwrapTLSConn(cl.Net.Conn)
	if !ok {
		return "", false
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		if cert.Subject.CommonName != "" {
			slog.Debug("Extracted CN from client certificate", "cn", cert.Subject.CommonName)
			return cert.Subject.CommonName, true
		}
		return "", true
	}
	return "", false
}

func unwrapTLSConn(conn net.Conn) (*tls.Conn, bool) {
	if conn == nil {
		return nil, false
	}

	if tlsConn, ok := conn.(*tls.Conn); ok {
		return tlsConn, true
	}

	return unwrapTLSConnValue(reflect.ValueOf(conn))
}

func unwrapTLSConnValue(v reflect.Value) (*tls.Conn, bool) {
	if !v.IsValid() {
		return nil, false
	}

	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, false
		}
		if tlsConn, ok := v.Interface().(*tls.Conn); ok {
			return tlsConn, true
		}
		v = v.Elem()
	}

	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil, false
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.IsValid() || !field.CanInterface() {
			continue
		}
		if tlsConn, ok := unwrapTLSConnValue(field); ok {
			return tlsConn, true
		}
	}

	return nil, false
}

func (a *authHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	if !pk.Connect.Clean {
		slog.Info("Mqtt client not authorized: things cannot use cleanSession false", "user", string(pk.Connect.Username), "clientId", pk.Connect.ClientIdentifier)
		return false
	}

	certCN, hasClientCert := extractCNFromCert(cl)
	result, ok := a.authzFn(connector.AuthContext{
		ClientIdentifier: pk.Connect.ClientIdentifier,
		Username:         string(pk.Connect.Username),
		Password:         string(pk.Connect.Password),
		HasClientCert:    hasClientCert,
		CertCN:           certCN,
	})
	if !ok {
		return false
	}
	cl.Properties.Username = []byte(result.Principal)
	return true
}

func (a *authHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	return a.aclFn(cl.ID, string(cl.Properties.Username), topic, write)
}
