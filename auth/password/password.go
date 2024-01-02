package password

import (
	"context"

	"ruff.io/tio/connector/mqtt/embed"

	"ruff.io/tio/config"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/thing"
)

func AuthzMqttClient(ctx context.Context, superUsers []config.UserPassword, thingSvc thing.Service) embed.AuthzFn {
	return func(connParams embed.ConnectParams) bool {
		user, password, clientId := string(connParams.Username), string(connParams.Password), connParams.ClientIdentifier
		for _, u := range superUsers {
			if user == u.Name && password == u.Password {
				log.Infof("Mqtt client user %s is authorized by default users", u.Name)
				return true
			}
		}
		th, err := thingSvc.Get(ctx, user)
		if err != nil {
			log.Infof("Mqtt client user %s client %s authz error: %v", user, clientId, err)
			return false
		}
		if th.AuthValue == password {
			if connParams.Clean == false {
				log.Warnf("Mqtt client user %s client %s authz error: things can not be allowed to use cleanSession false", user, clientId)
				return false
			}
			return true
		} else {
			log.Infof("Mqtt client user %s client %s password %s is not authorized", user, clientId, password)
			return false
		}
	}
}
