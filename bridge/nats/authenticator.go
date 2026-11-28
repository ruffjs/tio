package nats

import (
	"log/slog"

	"ruff.io/tio/config"

	server "github.com/nats-io/nats-server/v2/server"
)

// NatsAuthenticator authenticates the embedded server's internal APP/SYS
// clients and configured super users. Devices no longer connect to NATS, so
// there is no dynamic thing registration or MQTT topic permission mapping.
type NatsAuthenticator struct {
	superUsers []config.UserPassword
	appClient  config.NatsClientConfig
	sysClient  config.NatsClientConfig
	appAcc     *server.Account
	sysAcc     *server.Account
}

func NewNatsAuthenticator(superUsers []config.UserPassword, appClient, sysClient config.NatsClientConfig) *NatsAuthenticator {
	return &NatsAuthenticator{
		superUsers: superUsers,
		appClient:  appClient,
		sysClient:  sysClient,
	}
}

func (a *NatsAuthenticator) SetAccounts(appAcc, sysAcc *server.Account) {
	a.appAcc = appAcc
	a.sysAcc = sysAcc
}

func (a *NatsAuthenticator) Check(c server.ClientAuthentication) bool {
	opts := c.GetOpts()
	username := opts.Username
	password := opts.Password

	if username == a.appClient.User && password == a.appClient.Password {
		return a.registerAppUser(c)
	}
	if username == a.sysClient.User && password == a.sysClient.Password {
		return a.registerSysUser(c)
	}
	for _, su := range a.superUsers {
		if username == su.Name && password == su.Password {
			return a.registerSuperUser(c, su)
		}
	}
	return false
}

func (a *NatsAuthenticator) registerAppUser(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal APP client authenticated", "user", a.appClient.User)
	user := &server.User{
		Username: a.appClient.User,
		Password: a.appClient.Password,
		Account:  a.appAcc,
		Permissions: &server.Permissions{
			Publish: &server.SubjectPermission{
				Allow: []string{"$iothub.>", "$tio.>", "$JS.API.>", "$KV.>", "_INBOX.>", "tio.>"},
			},
			Subscribe: &server.SubjectPermission{
				Allow: []string{"$iothub.>", "$tio.>", "$JS.API.>", "_INBOX.>", "$KV.>", "tio.>"},
			},
		},
	}
	c.RegisterUser(user)
	return true
}

func (a *NatsAuthenticator) registerSysUser(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal SYS client authenticated", "user", a.sysClient.User)
	acc := a.sysAcc
	if acc == nil {
		return false
	}
	user := &server.User{
		Username: a.sysClient.User,
		Password: a.sysClient.Password,
		Account:  acc,
		Permissions: &server.Permissions{
			Publish: &server.SubjectPermission{
				Allow: []string{"$SYS.>", "$tio.>", "$iothub.>", "_INBOX.>"},
			},
			Subscribe: &server.SubjectPermission{
				Allow: []string{"$SYS.>", "$tio.events.>", "$tio.>", "_INBOX.>"},
			},
		},
	}
	c.RegisterUser(user)
	return true
}

func (a *NatsAuthenticator) registerSuperUser(c server.ClientAuthentication, su config.UserPassword) bool {
	slog.Debug("NATS super user authenticated", "user", su.Name)
	user := &server.User{
		Username: su.Name,
		Password: su.Password,
		Account:  a.appAcc,
		Permissions: &server.Permissions{
			Publish:   &server.SubjectPermission{Allow: []string{">"}},
			Subscribe: &server.SubjectPermission{Allow: []string{">"}},
		},
	}
	c.RegisterUser(user)
	return true
}
