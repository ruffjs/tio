package auth

import (
	"context"
	"log/slog"
	"strings"

	"ruff.io/tio/config"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type AclFn = func(username string, topic string, write bool) bool

type BindingGetter interface {
	IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error)
}

func TopicAcl(bg BindingGetter, superUsers []config.UserPassword) AclFn {
	return func(username string, topic string, write bool) bool {
		for _, u := range superUsers {
			if u.Name == username {
				return true
			}
		}
		thingTopicPrefix := shadow.TopicThingsPrefix + username + "/"
		userThingTopicPrefix := shadow.TopicUserThingsPrefix + username + "/"

		// For reserved topics
		if strings.HasPrefix(topic, shadow.TopicThingsPrefix) || strings.HasPrefix(topic, shadow.TopicUserThingsPrefix) {
			if strings.HasPrefix(topic, thingTopicPrefix) || strings.HasPrefix(topic, userThingTopicPrefix) {
				return true
			}
			// Check whether the current thing is bound to the gateway
			tingId, err := model.GetThingIdFromTopic(topic)
			if err != nil {
				slog.Error("Mqtt acl get thingId error", "thingId", username, "topic", topic, "error", err)
				return false
			}

			if bound, err := bg.IsBoundGateway(context.Background(), tingId, username); err == nil && bound {
				return true
			}

			op := "subscribe"
			if write {
				op = "publish"
			}
			slog.Debug("Mqqt acl deny", "op", op, "thingId", username, "topic", topic, "error", err)
			return false
		} else {
			// Non-reserved topics
			return true
		}
	}
}
