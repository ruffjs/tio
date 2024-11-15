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

func AuthzMqttClient(ctx context.Context, superUsers []config.UserPassword, thingSvc thing.Service, provision thing.Provision) embed.AuthzFn {
	return func(connParams embed.ConnectParams) bool {
		user, password, clientId := string(connParams.Username), string(connParams.Password), connParams.ClientIdentifier
		for _, u := range superUsers {
			if user == u.Name && password == u.Password {
				slog.Info("Mqtt client is authorized by default users", "user", u.Name)
				return true
			}
		}
		th, err := thingSvc.Get(ctx, user)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) && provision != nil {
				pass, _, err := provision.AutoRegisterViaHmac(ctx, user, password)
				if pass && err == nil {
					return true
				}
			}
			slog.Info("Mqtt client authz error", "user", user, "clientId", clientId, "error", err)
			return false
		}
		if !th.Enabled {
			slog.Info("Mqtt client is not authorized cause thing is not Enabled",
				"user", user, "clientId", clientId)
			return false
		}
		if th.AuthValue == password {
			if !connParams.Clean {
				slog.Warn("Mqtt client authz error: things can not be allowed to use cleanSession false",
					"user", user, "clientId", clientId)
				return false
			}
			return true
		} else {
			slog.Info("Mqtt client not authorized cause password is wrong",
				"user", user, "clientId", clientId, "password", password)
			return false
		}
	}
}
