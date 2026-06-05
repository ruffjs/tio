package embed

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/eventbus"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/stretchr/testify/require"
)

func TestEmbedBrokerWSSConnectivity(t *testing.T) {
	caTLS, caCert := mustCreateCA(t)
	serverTLS := mustCreateSignedCert(t, caTLS, caCert, false, "tio-server")
	certFile, keyFile, clientCAFile := writeServerTLSFiles(t, serverTLS, caCert)

	ctx, cancel := context.WithCancel(context.Background())
	wssPort := getFreePort(t)
	evtBus := eventbus.NewEventBus[connector.PresenceEvent]()
	svr := initBroker(ctx, MochiConfig{
		TcpPort:      getFreePort(t),
		WsPort:       getFreePort(t),
		WssPort:      wssPort,
		CertFile:     certFile,
		KeyFile:      keyFile,
		ClientCAFile: clientCAFile,
		AuthzFn: func(authCtx AuthContext) (AuthResult, bool) {
			return AuthResult{Principal: authCtx.Username}, true
		},
		AclFn: func(clientId, user string, topic string, write bool) bool {
			return true
		},
	}, evtBus)
	broker = &embedBroker{
		impl:             svr,
		presenceEventBus: evtBus,
		ctx:              ctx,
		cancel:           cancel,
	}

	require.NoError(t, svr.Serve())
	t.Cleanup(func() {
		cancel()
		_ = svr.Close()
		broker = nil
	})

	msgCh := make(chan packets.Packet, 1)
	require.NoError(t, svr.Subscribe("test/wss", 1, func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		msgCh <- pk
	}))

	opts := pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("wss://127.0.0.1:%d/", wssPort)).
		SetClientID("wss-test-client").
		SetUsername("wss-user").
		SetPassword("wss-pass").
		SetConnectTimeout(3 * time.Second).
		SetAutoReconnect(false).
		SetTLSConfig(&tls.Config{
			RootCAs:    mustCertPool(t, caCert),
			ServerName: "tio-server",
		})

	cl := pahomqtt.NewClient(opts)
	connectToken := cl.Connect()
	require.True(t, connectToken.WaitTimeout(5*time.Second), "wss connect timed out")
	require.NoError(t, connectToken.Error())
	require.True(t, cl.IsConnected())

	require.Eventually(t, func() bool {
		client, ok := svr.Clients.Get("wss-test-client")
		return ok && !client.Closed()
	}, 5*time.Second, 50*time.Millisecond)

	pubToken := cl.Publish("test/wss", 0, false, []byte("hello-wss"))
	require.True(t, pubToken.WaitTimeout(5*time.Second), "wss publish timed out")
	require.NoError(t, pubToken.Error())

	select {
	case pk := <-msgCh:
		require.Equal(t, "test/wss", pk.TopicName)
		require.Equal(t, []byte("hello-wss"), pk.Payload)
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive mqtt publish over wss")
	}

	cl.Disconnect(250)
	require.Eventually(t, func() bool {
		client, ok := svr.Clients.Get("wss-test-client")
		return !ok || client.Closed()
	}, 5*time.Second, 50*time.Millisecond)
}

func TestEmbedBrokerWSSClientCertificateAuth(t *testing.T) {
	caTLS, caCert := mustCreateCA(t)
	serverTLS := mustCreateSignedCert(t, caTLS, caCert, false, "tio-server")
	clientTLS := mustCreateSignedCert(t, caTLS, caCert, true, "thing-cert")
	certFile, keyFile, clientCAFile := writeServerTLSFiles(t, serverTLS, caCert)

	ctx, cancel := context.WithCancel(context.Background())
	wssPort := getFreePort(t)
	evtBus := eventbus.NewEventBus[connector.PresenceEvent]()
	svr := initBroker(ctx, MochiConfig{
		TcpPort:           getFreePort(t),
		WsPort:            getFreePort(t),
		WssPort:           wssPort,
		CertFile:          certFile,
		KeyFile:           keyFile,
		ClientCAFile:      clientCAFile,
		RequireClientCert: true,
		AuthzFn: func(authCtx AuthContext) (AuthResult, bool) {
			require.True(t, authCtx.HasClientCert)
			require.Equal(t, "thing-cert", authCtx.CertCN)
			return AuthResult{Principal: authCtx.CertCN}, true
		},
		AclFn: func(clientId, user string, topic string, write bool) bool {
			return true
		},
	}, evtBus)
	broker = &embedBroker{
		impl:             svr,
		presenceEventBus: evtBus,
		ctx:              ctx,
		cancel:           cancel,
	}

	require.NoError(t, svr.Serve())
	t.Cleanup(func() {
		cancel()
		_ = svr.Close()
		broker = nil
	})

	opts := pahomqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("wss://127.0.0.1:%d/", wssPort)).
		SetClientID("wss-mtls-client").
		SetConnectTimeout(3 * time.Second).
		SetAutoReconnect(false).
		SetTLSConfig(&tls.Config{
			Certificates: []tls.Certificate{clientTLS},
			RootCAs:      mustCertPool(t, caCert),
			ServerName:   "tio-server",
		})

	cl := pahomqtt.NewClient(opts)
	connectToken := cl.Connect()
	require.True(t, connectToken.WaitTimeout(5*time.Second), "wss mtls connect timed out")
	require.NoError(t, connectToken.Error())
	require.True(t, cl.IsConnected())

	cl.Disconnect(250)
}

func getFreePort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return addr.Port
}

func writeServerTLSFiles(t *testing.T, serverTLS tls.Certificate, caCert *x509.Certificate) (certFile string, keyFile string, clientCAFile string) {
	t.Helper()

	dir := t.TempDir()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverTLS.Certificate[0]})

	rsaKey, ok := serverTLS.PrivateKey.(*rsa.PrivateKey)
	require.True(t, ok)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})

	certFile = filepath.Join(dir, "server-cert.pem")
	keyFile = filepath.Join(dir, "server-key.pem")
	clientCAFile = filepath.Join(dir, "ca.pem")

	require.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))
	require.NoError(t, os.WriteFile(clientCAFile, caPEM, 0o600))

	return certFile, keyFile, clientCAFile
}
