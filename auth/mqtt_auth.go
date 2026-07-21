package auth

import (
	"context"
	"errors"
	"log/slog"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/thing"
)

// AuthzMqttClient authenticates MQTT clients with either password or client certificate.
func AuthzMqttClient(ctx context.Context, superUsers []config.UserPassword, thingSvc thing.Service, provision thing.Provision) connector.AuthzFn {
	return func(authCtx connector.AuthContext) (connector.AuthResult, bool) {
		if result, ok := authenticateSuperUser(authCtx, superUsers); ok {
			return result, true
		}

		if authCtx.HasClientCert {
			return authenticateByCertificate(ctx, authCtx, thingSvc)
		}

		return authenticateByPassword(ctx, authCtx, thingSvc, provision)
	}
}

func authenticateSuperUser(authCtx connector.AuthContext, superUsers []config.UserPassword) (connector.AuthResult, bool) {
	for _, u := range superUsers {
		if authCtx.Username == u.Name && authCtx.Password == u.Password {
			slog.Info("Mqtt client authorized by super user", "user", u.Name, "clientId", authCtx.ClientIdentifier)
			return connector.AuthResult{
				Principal:  u.Name,
				AuthMethod: "superuser-password",
			}, true
		}
	}
	return connector.AuthResult{}, false
}

func authenticateByCertificate(ctx context.Context, authCtx connector.AuthContext, thingSvc thing.Service) (connector.AuthResult, bool) {
	thingID := authCtx.CertCN
	if thingID == "" {
		logFailure("client certificate common name is empty", authCtx.Username, authCtx.ClientIdentifier)
		return connector.AuthResult{}, false
	}
	if authCtx.Username != "" && authCtx.Username != thingID {
		logFailure("username does not match client certificate common name", authCtx.Username, authCtx.ClientIdentifier, "certCN", thingID)
		return connector.AuthResult{}, false
	}

	th, err := thingSvc.Get(ctx, thingID)
	if err != nil {
		logFailure("thing not found", thingID, authCtx.ClientIdentifier, "error", err)
		return connector.AuthResult{}, false
	}

	if !th.Enabled {
		logFailure("thing is not enabled", thingID, authCtx.ClientIdentifier)
		return connector.AuthResult{}, false
	}
	if !thing.IsCertificateAuthType(th.AuthType) {
		logFailure("thing auth type does not allow certificate", thingID, authCtx.ClientIdentifier, "authType", th.AuthType)
		return connector.AuthResult{}, false
	}

	slog.Info("Mqtt client authorized via certificate", "thingId", thingID, "clientId", authCtx.ClientIdentifier)
	return connector.AuthResult{
		Principal:  thingID,
		AuthMethod: "certificate",
	}, true
}

func authenticateByPassword(ctx context.Context, authCtx connector.AuthContext, thingSvc thing.Service, provision thing.Provision) (connector.AuthResult, bool) {
	user := authCtx.Username
	th, err := thingSvc.Get(ctx, user)
	if err != nil {
		if result, ok := handleGetThingError(ctx, authCtx, err, provision); ok {
			return result, true
		}
		logFailure("thing not found", user, authCtx.ClientIdentifier, "error", err)
		return connector.AuthResult{}, false
	}

	if !th.Enabled {
		logFailure("thing is not enabled", user, authCtx.ClientIdentifier)
		return connector.AuthResult{}, false
	}
	if !thing.IsPasswordAuthType(th.AuthType) {
		logFailure("thing auth type does not allow password", user, authCtx.ClientIdentifier, "authType", th.AuthType)
		return connector.AuthResult{}, false
	}

	return authenticateWithThing(ctx, authCtx, th, thingSvc)
}

func handleGetThingError(ctx context.Context, authCtx connector.AuthContext, err error, provision thing.Provision) (connector.AuthResult, bool) {
	if provision != nil && errors.Is(err, model.ErrNotFound) {
		if pass, _, err := provision.AutoRegisterViaHmac(ctx, authCtx.Username, authCtx.Password); pass && err == nil {
			slog.Info("Mqtt client authorized via provision", "user", authCtx.Username, "clientId", authCtx.ClientIdentifier)
			return connector.AuthResult{
				Principal:  authCtx.Username,
				AuthMethod: "password-provision",
			}, true
		}
	}
	return connector.AuthResult{}, false
}

func authenticateWithThing(ctx context.Context, authCtx connector.AuthContext, th *thing.Thing, thingSvc thing.Service) (connector.AuthResult, bool) {
	user, password, clientID := authCtx.Username, authCtx.Password, authCtx.ClientIdentifier

	switch {
	case th.AuthValue == "" && password != "":
		if err := thingSvc.UpdateAuthValue(ctx, user, password); err != nil {
			slog.Error("Failed to update thing AuthValue", "user", user, "clientId", clientID, "error", err)
			return connector.AuthResult{}, false
		}
		slog.Info("Mqtt client authorized and AuthValue updated", "user", user, "clientId", clientID)
		return connector.AuthResult{
			Principal:  user,
			AuthMethod: "password-bootstrap",
		}, true
	case th.AuthValue == password:
		slog.Info("Mqtt client authorized", "user", user, "clientId", clientID)
		return connector.AuthResult{
			Principal:  user,
			AuthMethod: "password",
		}, true
	default:
		logFailure("password is wrong", user, clientID)
		return connector.AuthResult{}, false
	}
}

func logFailure(reason, user, clientId string, args ...any) {
	fields := []any{"user", user, "clientId", clientId}
	fields = append(fields, args...)
	slog.Info("Mqtt client not authorized: "+reason, fields...)
}
