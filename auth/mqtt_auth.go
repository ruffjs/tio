package auth

import (
	"context"
	"errors"
	"log/slog"

	"ruff.io/tio/config"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/namespace"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/thing"
)

// AuthzMqttClient authenticates MQTT clients with either password or client certificate.
func AuthzMqttClient(ctx context.Context, superUsers []config.UserPassword, thingSvc thing.Service, provision thing.Provision, namespaces ...[]config.Namespace) embed.AuthzFn {
	return func(authCtx embed.AuthContext) (embed.AuthResult, bool) {
		if result, ok := authenticateSuperUser(authCtx, superUsers); ok {
			return result, true
		}
		if len(namespaces) > 0 {
			if result, ok := authenticateNamespace(authCtx, namespaces[0]); ok {
				return result, true
			}
		}

		if !authCtx.Clean {
			logFailure("things cannot use cleanSession false", authCtx.Username, authCtx.ClientIdentifier)
			return embed.AuthResult{}, false
		}

		if authCtx.HasClientCert {
			return authenticateByCertificate(ctx, authCtx, thingSvc)
		}

		return authenticateByPassword(ctx, authCtx, thingSvc, provision)
	}
}

func authenticateSuperUser(authCtx embed.AuthContext, superUsers []config.UserPassword) (embed.AuthResult, bool) {
	for _, u := range superUsers {
		if authCtx.Username == u.Name && authCtx.Password == u.Password {
			slog.Info("Mqtt client authorized by super user", "user", u.Name, "clientId", authCtx.ClientIdentifier)
			return embed.AuthResult{
				Principal:  u.Name,
				AuthMethod: "superuser-password",
			}, true
		}
	}
	return embed.AuthResult{}, false
}

func authenticateNamespace(authCtx embed.AuthContext, namespaces []config.Namespace) (embed.AuthResult, bool) {
	if authCtx.HasClientCert {
		return embed.AuthResult{}, false
	}
	ns, ok := namespace.NamespaceForUser(namespaces, authCtx.Username, authCtx.Password)
	if !ok {
		return embed.AuthResult{}, false
	}
	slog.Info("Mqtt client authorized by namespace user", "user", authCtx.Username, "ns", ns, "clientId", authCtx.ClientIdentifier)
	return embed.AuthResult{
		Principal:  namespace.Principal(ns),
		AuthMethod: "namespace-password",
	}, true
}

func authenticateByCertificate(ctx context.Context, authCtx embed.AuthContext, thingSvc thing.Service) (embed.AuthResult, bool) {
	thingID := authCtx.CertCN
	if thingID == "" {
		logFailure("client certificate common name is empty", authCtx.Username, authCtx.ClientIdentifier)
		return embed.AuthResult{}, false
	}
	if authCtx.Username != "" && authCtx.Username != thingID {
		logFailure("username does not match client certificate common name", authCtx.Username, authCtx.ClientIdentifier, "certCN", thingID)
		return embed.AuthResult{}, false
	}

	th, err := thingSvc.Get(ctx, thingID)
	if err != nil {
		logFailure("thing not found", thingID, authCtx.ClientIdentifier, "error", err)
		return embed.AuthResult{}, false
	}

	if !th.Enabled {
		logFailure("thing is not enabled", thingID, authCtx.ClientIdentifier)
		return embed.AuthResult{}, false
	}
	if !thing.IsCertificateAuthType(th.AuthType) {
		logFailure("thing auth type does not allow certificate", thingID, authCtx.ClientIdentifier, "authType", th.AuthType)
		return embed.AuthResult{}, false
	}

	slog.Info("Mqtt client authorized via certificate", "thingId", thingID, "clientId", authCtx.ClientIdentifier)
	return embed.AuthResult{
		Principal:  thingID,
		AuthMethod: "certificate",
	}, true
}

func authenticateByPassword(ctx context.Context, authCtx embed.AuthContext, thingSvc thing.Service, provision thing.Provision) (embed.AuthResult, bool) {
	user := authCtx.Username
	th, err := thingSvc.Get(ctx, user)
	if err != nil {
		if result, ok := handleGetThingError(ctx, authCtx, err, provision); ok {
			return result, true
		}
		logFailure("thing not found", user, authCtx.ClientIdentifier, "error", err)
		return embed.AuthResult{}, false
	}

	if !th.Enabled {
		logFailure("thing is not enabled", user, authCtx.ClientIdentifier)
		return embed.AuthResult{}, false
	}
	if !thing.IsPasswordAuthType(th.AuthType) {
		logFailure("thing auth type does not allow password", user, authCtx.ClientIdentifier, "authType", th.AuthType)
		return embed.AuthResult{}, false
	}

	return authenticateWithThing(ctx, authCtx, th, thingSvc)
}

func handleGetThingError(ctx context.Context, authCtx embed.AuthContext, err error, provision thing.Provision) (embed.AuthResult, bool) {
	if provision != nil && errors.Is(err, model.ErrNotFound) {
		if pass, _, err := provision.AutoRegisterViaHmac(ctx, authCtx.Username, authCtx.Password); pass && err == nil {
			slog.Info("Mqtt client authorized via provision", "user", authCtx.Username, "clientId", authCtx.ClientIdentifier)
			return embed.AuthResult{
				Principal:  authCtx.Username,
				AuthMethod: "password-provision",
			}, true
		}
	}
	return embed.AuthResult{}, false
}

func authenticateWithThing(ctx context.Context, authCtx embed.AuthContext, th *thing.Thing, thingSvc thing.Service) (embed.AuthResult, bool) {
	user, password, clientID := authCtx.Username, authCtx.Password, authCtx.ClientIdentifier

	switch {
	case th.AuthValue == "" && password != "":
		if err := thingSvc.UpdateAuthValue(ctx, user, password); err != nil {
			slog.Error("Failed to update thing AuthValue", "user", user, "clientId", clientID, "error", err)
			return embed.AuthResult{}, false
		}
		slog.Info("Mqtt client authorized and AuthValue updated", "user", user, "clientId", clientID)
		return embed.AuthResult{
			Principal:  user,
			AuthMethod: "password-bootstrap",
		}, true
	case th.AuthValue == password:
		slog.Info("Mqtt client authorized", "user", user, "clientId", clientID)
		return embed.AuthResult{
			Principal:  user,
			AuthMethod: "password",
		}, true
	default:
		logFailure("password is wrong", user, clientID)
		return embed.AuthResult{}, false
	}
}

func logFailure(reason, user, clientId string, args ...any) {
	fields := []any{"user", user, "clientId", clientId}
	fields = append(fields, args...)
	slog.Info("Mqtt client not authorized: "+reason, fields...)
}
