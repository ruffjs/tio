package nats

import (
	"testing"
	"time"

	"ruff.io/tio/config"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

const (
	testAppUser = "app-user"
	testAppPass = "app-pass"
	testSysUser = "sys-user"
	testSysPass = "sys-pass"
)

func TestNatsAuthenticator_AppClientAuth(t *testing.T) {
	authn := NewNatsAuthenticator(nil,
		config.NatsClientConfig{User: testAppUser, Password: testAppPass},
		config.NatsClientConfig{User: testSysUser, Password: testSysPass})
	s := startTestServer(t, authn)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo(testAppUser, testAppPass), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	require.NoError(t, nc.Publish("tio.dev1.data", []byte("up")))
	require.NoError(t, nc.Flush())
}

func TestNatsAuthenticator_SysClientAuth(t *testing.T) {
	authn := NewNatsAuthenticator(nil,
		config.NatsClientConfig{User: testAppUser, Password: testAppPass},
		config.NatsClientConfig{User: testSysUser, Password: testSysPass})
	s := startTestServer(t, authn)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo(testSysUser, testSysPass), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	_, err = nc.SubscribeSync("$SYS.>")
	require.NoError(t, err)
}

func TestNatsAuthenticator_SuperUserAuth(t *testing.T) {
	authn := NewNatsAuthenticator([]config.UserPassword{{Name: "admin", Password: "secret"}},
		config.NatsClientConfig{User: testAppUser, Password: testAppPass},
		config.NatsClientConfig{User: testSysUser, Password: testSysPass})
	s := startTestServer(t, authn)

	nc, err := nats.Connect(s.ClientURL(), nats.UserInfo("admin", "secret"), nats.Timeout(2*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	sub, err := nc.SubscribeSync(">")
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	require.NoError(t, nc.Publish("business.custom", []byte("x")))
	msg, err := sub.NextMsg(time.Second)
	require.NoError(t, err)
	require.Equal(t, "business.custom", msg.Subject)
	require.Equal(t, []byte("x"), msg.Data)
}

func TestNatsAuthenticator_UnknownRejected(t *testing.T) {
	authn := NewNatsAuthenticator(nil,
		config.NatsClientConfig{User: testAppUser, Password: testAppPass},
		config.NatsClientConfig{User: testSysUser, Password: testSysPass})
	s := startTestServer(t, authn)

	_, err := nats.Connect(s.ClientURL(), nats.UserInfo("bad-user", "bad-pass"), nats.Timeout(2*time.Second))
	require.Error(t, err)
}

func TestNatsAuthenticator_AnonymousRejected(t *testing.T) {
	authn := NewNatsAuthenticator(nil,
		config.NatsClientConfig{User: testAppUser, Password: testAppPass},
		config.NatsClientConfig{User: testSysUser, Password: testSysPass})
	s := startTestServer(t, authn)

	_, err := nats.Connect(s.ClientURL(), nats.Name("anon"), nats.Timeout(2*time.Second))
	require.Error(t, err)
}

func startTestServer(t *testing.T, authn *NatsAuthenticator) *NatsServer {
	t.Helper()
	s, err := StartNatsServer(config.NatsServerConfig{
		ServerName:  "test-" + t.Name(),
		ClusterName: "test-cluster",
		Port:        -1,
		MonitorPort: 0,
	}, JetStreamOptions{}, authn)
	require.NoError(t, err)
	t.Cleanup(s.Shutdown)

	appAcc, err := s.Server().LookupAccount(AppAccountName)
	require.NoError(t, err)
	sysAcc, err := s.Server().LookupAccount(SysAccountName)
	require.NoError(t, err)
	authn.SetAccounts(appAcc, sysAcc)
	return s
}
