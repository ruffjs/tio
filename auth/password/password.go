package password

import (
	"context"

	"ruff.io/tio/connector/mqtt/embed"

	"ruff.io/tio/config"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/thing"
)

func AuthzMqttClient(ctx context.Context, superUsers []config.UserPassword, thingSvc thing.Service) embed.AuthzFn {
	return func(user, password string) bool {
		for _, u := range superUsers {
			if user == u.Name && password == u.Password {
				log.Infof("Mqtt client user %s is authorized by default users", u.Name)
				return true
			}
		}
		th, err := thingSvc.Get(ctx, user)
		if err != nil {
			log.Infof("Mqtt client user %s authz error: %v", user, err)
			return false
		}
		if th.AuthValue == password {
			return true
		} else {
			log.Infof("Mqtt client user %s password %s is not authorized", user, password)
			return false
		}
	}
}
