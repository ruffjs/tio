package auth

import (
	"context"
	"log/slog"
	"strings"

	"ruff.io/tio/config"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type AclFn = func(clientId, username string, topic string, write bool) bool

type BindingGetter interface {
	IsBoundGateway(ctx context.Context, thingId, gatewayThingId string) (bool, error)
}

// TODO Optimize: Prevent device connection if it has exceeded the maximum number of allowed operations without an Access Control List (ACL)
func TopicAcl(bg BindingGetter, superUsers []config.UserPassword) AclFn {
	return func(clientId, username string, topic string, write bool) bool {
		// Embeded MQTT inline client username is empty
		if username == "" {
			slog.Warn("Unexpected username is empty", "clientId", clientId, "topic", topic, "write", write)
			return true
		}
		for _, u := range superUsers {
			if u.Name == username {
				return true
			}
		}
		thingTopicPrefix := shadow.TopicThingsPrefix + username + "/"
		userThingTopicPrefix := shadow.TopicUserThingsPrefix + username + "/"

		// For thing's self reserved topics
		if strings.HasPrefix(topic, thingTopicPrefix) || strings.HasPrefix(topic, userThingTopicPrefix) {
			return true
		}
		// For other reserved topics
		if strings.HasPrefix(topic, shadow.TopicThingsPrefix) || strings.HasPrefix(topic, shadow.TopicUserThingsPrefix) {
			tingId, err := model.GetThingIdFromTopic(topic)
			if err != nil {
				slog.Error("Mqtt acl get thingId error", "thingId", username, "topic", topic, "error", err)
				return false
			}
			// Check whether the current thing is bound to the gateway
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
