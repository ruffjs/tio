package embed

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/connector"
)

func TestAuthHookUsesCertificateCNAsPrincipal(t *testing.T) {
	caTLS, caCert := mustCreateCA(t)
	serverTLS := mustCreateSignedCert(t, caTLS, caCert, false, "tio-server")
	clientTLS := mustCreateSignedCert(t, caTLS, caCert, true, "thing-cert")

	serverConn, clientConn := net.Pipe()
	serverTLSConn := tls.Server(serverConn, &tls.Config{
		Certificates: []tls.Certificate{serverTLS},
		ClientCAs:    mustCertPool(t, caCert),
		ClientAuth:   tls.RequireAndVerifyClientCert,
	})
	clientTLSConn := tls.Client(clientConn, &tls.Config{
		Certificates: []tls.Certificate{clientTLS},
		RootCAs:      mustCertPool(t, caCert),
		ServerName:   "tio-server",
	})

	errCh := make(chan error, 2)
	go func() { errCh <- serverTLSConn.Handshake() }()
	go func() { errCh <- clientTLSConn.Handshake() }()
	require.NoError(t, <-errCh)
	require.NoError(t, <-errCh)
	t.Cleanup(func() {
		_ = serverTLSConn.Close()
		_ = clientTLSConn.Close()
	})

	hook := &authHook{
		authzFn: func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
			require.True(t, authCtx.HasClientCert)
			require.Equal(t, "thing-cert", authCtx.CertCN)
			return connector.AuthResult{Principal: authCtx.CertCN}, true
		},
		aclFn: func(clientId, user string, topic string, write bool) bool { return true },
	}

	cl := &mqtt.Client{
		ID: "cid-1",
		Net: mqtt.ClientConnection{
			Conn: serverTLSConn,
		},
	}
	pk := packets.Packet{
		Connect: packets.ConnectParams{
			ClientIdentifier: "cid-1",
			Username:         []byte("raw-user"),
			Clean:            true,
		},
	}

	ok := hook.OnConnectAuthenticate(cl, pk)
	require.True(t, ok)
	require.Equal(t, "thing-cert", string(cl.Properties.Username))
}

func mustCreateCA(t *testing.T) (tls.Certificate, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test-ca",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	parsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, parsed
}

func mustCreateSignedCert(t *testing.T, caTLS tls.Certificate, caCert *x509.Certificate, isClient bool, cn string) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
	}
	if isClient {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		tpl.DNSNames = []string{cn}
	}

	caPrivateKey, ok := caTLS.PrivateKey.(*rsa.PrivateKey)
	require.True(t, ok)
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &key.PublicKey, caPrivateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return cert
}

func mustCertPool(t *testing.T, cert *x509.Certificate) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool
}
