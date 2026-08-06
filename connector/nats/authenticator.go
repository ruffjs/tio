package nats

import (
	"context"
	"log/slog"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"

	server "github.com/nats-io/nats-server/v2/server"
)

type NatsAuthenticator struct {
	authzFn       connector.AuthzFn
	bindingGetter connector.BindingGetter
	superUsers    []config.UserPassword
	appClient     config.NatsClientConfig
	sysClient     config.NatsClientConfig
	mqttPublisher config.NatsClientConfig
	appAcc        *server.Account
	sysAcc        *server.Account
	protocolMode  string
}

func NewNatsAuthenticator(
	authzFn connector.AuthzFn,
	bindingGetter connector.BindingGetter,
	superUsers []config.UserPassword,
	appClient, sysClient, mqttPublisher config.NatsClientConfig,
	protocolMode string,
) *NatsAuthenticator {
	return &NatsAuthenticator{
		authzFn:       authzFn,
		bindingGetter: bindingGetter,
		superUsers:    superUsers,
		appClient:     appClient,
		sysClient:     sysClient,
		mqttPublisher: mqttPublisher,
		protocolMode:  protocolMode,
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

func (a *NatsAuthenticator) registerInternalSysUser(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal SYS client authenticated", "user", a.sysClient.User)
	acc := a.sysAcc
	if acc == nil {
		acc = a.appAcc
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

func (a *NatsAuthenticator) registerInternalMqttPublisher(c server.ClientAuthentication) bool {
	slog.Debug("NATS internal MQTT publisher authenticated", "user", a.mqttPublisher.User)
	user := &server.User{
		Username: a.mqttPublisher.User,
		Password: a.mqttPublisher.Password,
		Account:  a.appAcc,
		Permissions: &server.Permissions{
			Publish: &server.SubjectPermission{
				Allow: []string{"$iothub.>", "tio.>"},
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
			allow := []string{"$iothub.>"}
			if a.protocolMode == "simple" {
				allow = []string{"tio.>"}
			}
			return &server.Permissions{
				Publish: &server.SubjectPermission{
					Allow: allow,
				},
				Subscribe: &server.SubjectPermission{
					Allow: append(append([]string{}, allow...), "$MQTT.sub.>", "_INBOX.>"),
				},
			}
		}
	}

	var pubAllow, pubDeny, subAllow, subDeny []string
	if a.protocolMode == "simple" {
		thingPrefix := "tio." + thingId + ".>"
		downExact := "tio." + thingId + ".down"
		downWild := "tio." + thingId + ".down.>"
		upExact := "tio." + thingId + ".up"
		upWild := "tio." + thingId + ".up.>"

		pubAllow = []string{thingPrefix}
		pubDeny = []string{downExact, downWild}
		subAllow = []string{thingPrefix, "$MQTT.sub.>"}
		subDeny = []string{upExact, upWild}
	} else {
		thingPrefix := "$iothub.things." + thingId + ".>"
		userPrefix := "$iothub.user.things." + thingId + ".>"
		pubAllow = []string{thingPrefix, userPrefix}
		subAllow = []string{thingPrefix, userPrefix, "$MQTT.sub.>", "$iothub.events.things.>"}
	}

	if a.bindingGetter != nil {
		boundIds, err := a.bindingGetter.GetBoundThingIds(context.Background(), thingId)
		if err != nil {
			slog.Error("get bound things for gateway permissions", "gateway", thingId, "error", err)
		} else {
			for _, bid := range boundIds {
				if a.protocolMode == "simple" {
					boundPrefix := "tio." + bid + ".>"
					boundDownExact := "tio." + bid + ".down"
					boundDownWild := "tio." + bid + ".down.>"
					boundUpExact := "tio." + bid + ".up"
					boundUpWild := "tio." + bid + ".up.>"

					pubAllow = append(pubAllow, boundPrefix)
					pubDeny = append(pubDeny, boundDownExact, boundDownWild)
					subAllow = append(subAllow, boundPrefix)
					subDeny = append(subDeny, boundUpExact, boundUpWild)
				} else {
					boundPub := "$iothub.things." + bid + ".>"
					boundUser := "$iothub.user.things." + bid + ".>"
					pubAllow = append(pubAllow, boundPub, boundUser)
					subAllow = append(subAllow, boundPub, boundUser)
				}
			}
		}
	}

	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: pubAllow,
			Deny:  pubDeny,
		},
		Subscribe: &server.SubjectPermission{
			Allow: subAllow,
			Deny:  subDeny,
		},
	}
}
