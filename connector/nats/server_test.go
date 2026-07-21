package nats

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestServerStartsAndIsRunning(t *testing.T) {
	cfg := testServerConfig(t)
	s, err := StartNatsServer(cfg, &allowAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	if !s.IsRunning() {
		t.Fatal("server should be running")
	}
}

func TestServerAccountsExist(t *testing.T) {
	cfg := testServerConfig(t)
	s, err := StartNatsServer(cfg, &allowAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	srv := s.Server()
	appAcc, err := srv.LookupAccount(AppAccountName)
	if err != nil {
		t.Fatalf("APP account not found: %v", err)
	}
	sysAcc, err := srv.LookupAccount(SysAccountName)
	if err != nil {
		t.Fatalf("SYS account not found: %v", err)
	}

	if appAcc.GetName() != AppAccountName {
		t.Fatalf("APP account name = %q, want %q", appAcc.GetName(), AppAccountName)
	}
	if sysAcc.GetName() != SysAccountName {
		t.Fatalf("SYS account name = %q, want %q", sysAcc.GetName(), SysAccountName)
	}
}

func TestAccountIsolation(t *testing.T) {
	cfg := testServerConfig(t)
	authn := &accountAuth{}
	s, err := StartNatsServer(cfg, authn)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.appAcc = appAcc
	authn.sysAcc = sysAcc

	ncApp, err := nats.Connect(s.ClientURL(), nats.UserInfo("app", "app"), nats.Name("app-client"))
	if err != nil {
		t.Fatalf("failed to connect as app: %v", err)
	}
	defer ncApp.Close()

	ncSys, err := nats.Connect(s.ClientURL(), nats.UserInfo("sys", "sys"), nats.Name("sys-client"))
	if err != nil {
		t.Fatalf("failed to connect as sys: %v", err)
	}
	defer ncSys.Close()

	received := make(chan *nats.Msg, 1)
	_, err = ncSys.Subscribe("test.subject", func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	ncSys.Flush()

	err = ncApp.Publish("test.subject", []byte("hello"))
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	select {
	case <-received:
		t.Fatal("SYS account should NOT receive messages from APP account")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAnonymousRejected(t *testing.T) {
	cfg := testServerConfig(t)
	s, err := StartNatsServer(cfg, &rejectAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	_, err = nats.Connect(s.ClientURL(), nats.Name("anon"), nats.Timeout(2*time.Second))
	if err == nil {
		t.Fatal("expected connection to be rejected, but it succeeded")
	}
}

func TestJetStreamAvailable(t *testing.T) {
	cfg := testServerConfig(t)
	authn := &accountAuth{}
	s, err := StartNatsServer(cfg, authn)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	appAcc, _ := s.Server().LookupAccount(AppAccountName)
	sysAcc, _ := s.Server().LookupAccount(SysAccountName)
	authn.appAcc = appAcc
	authn.sysAcc = sysAcc

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo("app", "app"), nats.Name("js-client"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("failed to get JetStream context: %v", err)
	}

	streamName := fmt.Sprintf("TEST_%d", time.Now().UnixNano())
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"test.js.>"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}

	_, err = js.Publish("test.js.hello", []byte("world"))
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	msg, err := js.GetLastMsg(streamName, "test.js.hello")
	if err != nil {
		t.Fatalf("failed to get message: %v", err)
	}
	if string(msg.Data) != "world" {
		t.Fatalf("message data = %q, want %q", string(msg.Data), "world")
	}

	err = js.DeleteStream(streamName)
	if err != nil {
		t.Fatalf("failed to delete stream: %v", err)
	}
}

func TestServerName(t *testing.T) {
	cfg := testServerConfig(t)
	cfg.ServerName = "my-test-server"

	s, err := StartNatsServer(cfg, &allowAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	if s.Server().Name() != "my-test-server" {
		t.Fatalf("ServerName = %q, want %q", s.Server().Name(), "my-test-server")
	}
}

func TestStoreDirUsed(t *testing.T) {
	cfg := testServerConfig(t)
	s, err := StartNatsServer(cfg, &allowAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	expected := filepath.Join(cfg.StoreDir, "jetstream")
	if s.Server().StoreDir() != expected {
		t.Fatalf("StoreDir = %q, want %q", s.Server().StoreDir(), expected)
	}
}

func TestCleanShutdown(t *testing.T) {
	cfg := testServerConfig(t)
	s, err := StartNatsServer(cfg, &allowAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	if !s.IsRunning() {
		t.Fatal("server should be running before shutdown")
	}

	done := make(chan struct{})
	go func() {
		s.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timed out")
	}

	if s.IsRunning() {
		t.Fatal("server should not be running after shutdown")
	}
}

func TestRestartWithSameStoreDir(t *testing.T) {
	storeDir := t.TempDir()
	authn1 := &accountAuth{}

	cfg := testServerConfig(t)
	cfg.StoreDir = storeDir

	s1, err := StartNatsServer(cfg, authn1)
	if err != nil {
		t.Fatalf("failed to start first server: %v", err)
	}

	appAcc1, _ := s1.Server().LookupAccount(AppAccountName)
	sysAcc1, _ := s1.Server().LookupAccount(SysAccountName)
	authn1.appAcc = appAcc1
	authn1.sysAcc = sysAcc1

	nc, err := nats.Connect(s1.ClientURL(), nats.UserInfo("app", "app"), nats.Name("persist-client"))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("failed to get JetStream context: %v", err)
	}

	streamName := fmt.Sprintf("PERSIST_%d", time.Now().UnixNano())
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"persist.>"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("failed to create stream: %v", err)
	}

	_, err = js.Publish("persist.data", []byte("persistent-data"))
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	nc.Close()
	s1.Shutdown()

	authn2 := &accountAuth{}
	cfg2 := testServerConfig(t)
	cfg2.StoreDir = storeDir

	s2, err := StartNatsServer(cfg2, authn2)
	if err != nil {
		t.Fatalf("failed to start second server: %v", err)
	}
	defer s2.Shutdown()

	appAcc2, _ := s2.Server().LookupAccount(AppAccountName)
	sysAcc2, _ := s2.Server().LookupAccount(SysAccountName)
	authn2.appAcc = appAcc2
	authn2.sysAcc = sysAcc2

	nc2, err := nats.Connect(s2.ClientURL(), nats.UserInfo("app", "app"), nats.Name("persist-client-2"))
	if err != nil {
		t.Fatalf("failed to connect after restart: %v", err)
	}
	defer nc2.Close()

	js2, err := nc2.JetStream()
	if err != nil {
		t.Fatalf("failed to get JetStream context after restart: %v", err)
	}

	msg, err := js2.GetLastMsg(streamName, "persist.data")
	if err != nil {
		t.Fatalf("failed to get persistent message after restart: %v", err)
	}
	if string(msg.Data) != "persistent-data" {
		t.Fatalf("message data = %q, want %q", string(msg.Data), "persistent-data")
	}
}

func TestMqttGatewayPortListening(t *testing.T) {
	cfg := testServerConfig(t)
	cfg.MqttPort = -1

	s, err := StartNatsServer(cfg, &allowAllAuth{})
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown()

	v, err := s.Server().Varz(nil)
	if err != nil {
		t.Fatalf("failed to get Varz: %v", err)
	}

	mqttPort := v.MQTT.Port
	if mqttPort <= 0 {
		t.Fatal("MQTT port is not assigned - MQTT gateway not listening")
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", mqttPort), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to MQTT port: %v", err)
	}
	conn.Close()
}
