package nats

import (
	"testing"

	"ruff.io/tio/config"

	server "github.com/nats-io/nats-server/v2/server"
)

func testServerConfig(t *testing.T) config.NatsServerConfig {
	t.Helper()
	return config.NatsServerConfig{
		ServerName:         "test-" + t.Name(),
		ClusterName:        "test-cluster",
		Port:               -1,
		ClusterPort:        0,
		MqttPort:           -1,
		WsPort:             0,
		StoreDir:           t.TempDir(),
		JetStreamMaxMemory: 64 * 1024 * 1024,
		JetStreamMaxStore:  128 * 1024 * 1024,
		MqttStreamReplicas: 1,
		PresenceReplicas:   1,
	}
}

type allowAllAuth struct{}

func (a *allowAllAuth) Check(c server.ClientAuthentication) bool { return true }

type rejectAllAuth struct{}

func (a *rejectAllAuth) Check(c server.ClientAuthentication) bool { return false }

type accountAuth struct {
	appAcc *server.Account
	sysAcc *server.Account
}

func (a *accountAuth) Check(c server.ClientAuthentication) bool {
	opts := c.GetOpts()
	switch opts.Username {
	case "app":
		c.RegisterUser(&server.User{Username: "app", Password: "app", Account: a.appAcc})
		return true
	case "sys":
		c.RegisterUser(&server.User{Username: "sys", Password: "sys", Account: a.sysAcc})
		return true
	}
	return false
}
