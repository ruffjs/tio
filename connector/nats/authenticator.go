package nats

import (
	"log/slog"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"

	server "github.com/nats-io/nats-server/v2/server"
)

const (
	internalAppUser         = "$tio-app"
	internalSysUser         = "$tio-sys"
	internalMqttPubUser     = "$tio-mqtt-publisher"
)

type NatsAuthenticator struct {
	authzFn       connector.AuthzFn
	aclFn         connector.AclFn
	superUsers    []config.UserPassword
	appClient     config.NatsClientConfig
	sysClient     config.NatsClientConfig
	mqttPublisher config.NatsClientConfig
	appAcc        *server.Account
	sysAcc        *server.Account
}

func NewNatsAuthenticator(
	authzFn connector.AuthzFn,
	aclFn connector.AclFn,
	superUsers []config.UserPassword,
	appClient, sysClient, mqttPublisher config.NatsClientConfig,
) *NatsAuthenticator {
	return &NatsAuthenticator{
		authzFn:       authzFn,
		aclFn:         aclFn,
		superUsers:    superUsers,
		appClient:     appClient,
		sysClient:     sysClient,
		mqttPublisher: mqttPublisher,
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

	if a.isInternalAppUser(username, password) {
		return a.registerInternalAppUser(c)
	}
	if a.isInternalSysUser(username, password) {
		return a.registerInternalSysUser(c)
	}
	if a.isInternalMqttPublisher(username, password) {
		return a.registerInternalMqttPublisher(c)
	}

	return a.registerDynamicThing(c, username, password)
}

func (a *NatsAuthenticator) isInternalAppUser(username, password string) bool {
	return username == a.appClient.User && password == a.appClient.Password
}

func (a *NatsAuthenticator) isInternalSysUser(username, password string) bool {
	return username == a.sysClient.User && password == a.sysClient.Password
}

func (a *NatsAuthenticator) isInternalMqttPublisher(username, password string) bool {
	return username == a.mqttPublisher.User && password == a.mqttPublisher.Password
}

func (a *NatsAuthenticator) registerInternalAppUser(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal APP client authenticated", "user", a.appClient.User)
	user := &server.User{
		Username:  a.appClient.User,
		Password:  a.appClient.Password,
		Account:   a.appAcc,
		Permissions: &server.Permissions{
			Publish: &server.SubjectPermission{
				Allow: []string{"$iothub.>", "$tio.>", "$JS.API.>", "$KV.>"},
				Deny:  []string{"$tio.control.>"},
			},
			Subscribe: &server.SubjectPermission{
				Allow: []string{"$iothub.>", "$tio.>", "$JS.API.>", "_INBOX.>", "$KV.>"},
				Deny:  []string{"$tio.control.>"},
			},
		},
	}
	c.RegisterUser(user)
	return true
}

func (a *NatsAuthenticator) registerInternalSysUser(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal SYS client authenticated", "user", a.sysClient.User)
	user := &server.User{
		Username:  a.sysClient.User,
		Password:  a.sysClient.Password,
		Account:   a.sysAcc,
		Permissions: &server.Permissions{
			Subscribe: &server.SubjectPermission{
				Allow: []string{"$SYS.>", "$tio.events.>"},
			},
		},
	}
	c.RegisterUser(user)
	return true
}

func (a *NatsAuthenticator) registerInternalMqttPublisher(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal MQTT publisher authenticated", "user", a.mqttPublisher.User)
	user := &server.User{
		Username:  a.mqttPublisher.User,
		Password:  a.mqttPublisher.Password,
		Account:   a.appAcc,
		Permissions: &server.Permissions{
			Publish: &server.SubjectPermission{
				Allow: []string{"$iothub.>"},
			},
			Subscribe: &server.SubjectPermission{
				Deny: []string{">"},
			},
		},
	}
	c.RegisterUser(user)
	return true
}

func (a *NatsAuthenticator) registerDynamicThing(c server.ClientAuthentication, username, password string) bool {
	if username == "" && password == "" {
		slog.Debug("NATS anonymous connection rejected")
		return false
	}

	certCN := a.extractCertCN(c)
	hasClientCert := certCN != ""

	authCtx := connector.AuthContext{
		ClientIdentifier: username,
		Username:         username,
		Password:         password,
		HasClientCert:    hasClientCert,
		CertCN:           certCN,
	}

	result, ok := a.authzFn(authCtx)
	if !ok {
		slog.Info("NATS dynamic thing auth failed", "user", username)
		return false
	}

	thingId := result.Principal
	slog.Info("NATS dynamic thing authenticated", "thingId", thingId, "method", result.AuthMethod)

	perms := a.thingPermissions(thingId)
	user := &server.User{
		Username:    thingId,
		Password:    password,
		Account:     a.appAcc,
		Permissions: perms,
	}
	c.RegisterUser(user)
	return true
}

func (a *NatsAuthenticator) extractCertCN(c server.ClientAuthentication) string {
	tlsState := c.GetTLSConnectionState()
	if tlsState == nil {
		return ""
	}
	if len(tlsState.PeerCertificates) == 0 {
		return ""
	}
	cert := tlsState.PeerCertificates[0]
	return cert.Subject.CommonName
}

func (a *NatsAuthenticator) thingPermissions(thingId string) *server.Permissions {
	for _, su := range a.superUsers {
		if su.Name == thingId {
			return &server.Permissions{
				Publish: &server.SubjectPermission{
					Allow: []string{"$iothub.>"},
				},
				Subscribe: &server.SubjectPermission{
					Allow: []string{"$iothub.>", "$MQTT.sub.>"},
				},
			}
		}
	}

	thingPrefix := "$iothub.things." + thingId + ".>"
	userPrefix := "$iothub.user.things." + thingId + ".>"

	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{thingPrefix, userPrefix},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{thingPrefix, userPrefix, "$MQTT.sub.>", "$iothub.events.things.>"},
		},
	}
}
