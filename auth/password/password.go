package password

import (
	"context"
	"errors"
	"log/slog"

	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/pkg/model"

	"ruff.io/tio/config"
	"ruff.io/tio/thing"
)

// AuthzMqttClient creates an MQTT client authorization function
func AuthzMqttClient(ctx context.Context, superUsers []config.UserPassword, thingSvc thing.Service, provision thing.Provision) embed.AuthzFn {
	return func(connParams embed.ConnectParams) bool {
		user, password, clientId := string(connParams.Username), string(connParams.Password), connParams.ClientIdentifier

		// Check super users first
		if checkSuperUser(user, password, superUsers) {
			return true
		}

		// Thing cannot use cleanSession false
		if !connParams.Clean {
			logFailure("things cannot use cleanSession false", user, clientId)
			return false
		}

		// Get and validate thing
		th, err := thingSvc.Get(ctx, user)
		if err != nil {
			return handleGetThingError(ctx, user, password, clientId, err, provision)
		}

		if !th.Enabled {
			logFailure("thing is not enabled", user, clientId)
			return false
		}

		// Authenticate thing
		return authenticateWithThing(ctx, user, password, clientId, th, thingSvc)
	}
}

// checkSuperUser validates super user credentials
func checkSuperUser(user, password string, superUsers []config.UserPassword) bool {
	for _, u := range superUsers {
		if user == u.Name && password == u.Password {
			slog.Info("Mqtt client authorized by super user", "user", u.Name)
			return true
		}
	}
	return false
}

// handleGetThingError handles errors when getting thing from service
func handleGetThingError(ctx context.Context, user, password, clientId string, err error, provision thing.Provision) bool {
	if provision != nil && errors.Is(err, model.ErrNotFound) {
		if pass, _, err := provision.AutoRegisterViaHmac(ctx, user, password); pass && err == nil {
			return true
		}
	}
	logFailure("thing not found", user, clientId, "error", err)
	return false
}

// authenticateWithThing handles password authentication for a thing
func authenticateWithThing(ctx context.Context, user, password, clientId string, th *thing.Thing, thingSvc thing.Service) bool {
	switch {
	case th.AuthValue == "" && password != "":
		// Auto-set password for empty AuthValue
		if err := thingSvc.UpdateAuthValue(ctx, user, password); err != nil {
			slog.Error("Failed to update thing AuthValue", "user", user, "clientId", clientId, "error", err)
			return false
		}
		slog.Info("Mqtt client authorized and AuthValue updated", "user", user, "clientId", clientId)
		return true

	case th.AuthValue == password:
		// Password matches
		slog.Info("Mqtt client authorized", "user", user, "clientId", clientId)
		return true

	default:
		// Password mismatch
		logFailure("password is wrong", user, clientId, "password", password)
		return false
	}
}

// logFailure logs authentication failure with consistent format
func logFailure(reason, user, clientId string, args ...any) {
	fields := []any{"user", user, "clientId", clientId}
	fields = append(fields, args...)
	slog.Info("Mqtt client not authorized: "+reason, fields...)
}
