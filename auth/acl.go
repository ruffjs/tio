package auth

import (
	"context"
	"log/slog"
	"strings"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
)

type AclFn = func(clientId, username string, topic string, write bool) bool

// TODO Optimize: Prevent device connection if it has exceeded the maximum number of allowed operations without an Access Control List (ACL)
func TopicAcl(bg connector.BindingGetter, superUsers []config.UserPassword, protocolMode string) AclFn {
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

		// Check if it's a simple protocol topic
		if strings.HasPrefix(topic, protocol.TopicSimplePrefix) {
			if protocolMode != "simple" {
				slog.Debug("Simple protocol topic rejected in legacy mode", "thingId", username, "topic", topic)
				return false
			}

			thingId, direction, err := protocol.ParseTopicDir(topic)
			if err != nil {
				slog.Error("Parse simple topic error", "thingId", username, "topic", topic, "error", err)
				return false
			}

			// Check if thing matches username or is bound to gateway
			if thingId != username {
				if bound, err := bg.IsBoundGateway(context.Background(), thingId, username); err != nil || !bound {
					slog.Debug("Simple topic thingId mismatch", "thingId", username, "topic", topic, "targetThingId", thingId)
					return false
				}
			}

			// Deny-list: only restrict known directions, allow everything else
			// under tio/{thingId}/# for free messaging via NATS.
			if write {
				if direction == protocol.LevelDown {
					slog.Debug("Simple protocol publish denied", "thingId", username, "topic", topic, "direction", direction)
					return false
				}
			} else {
				if direction == protocol.LevelUp {
					slog.Debug("Simple protocol subscribe denied", "thingId", username, "topic", topic, "direction", direction)
					return false
				}
			}

			return true
		}

		// Legacy protocol topics
		if protocolMode == "simple" {
			// Reject legacy topics in simple mode
			if strings.HasPrefix(topic, shadow.TopicThingsPrefix) || strings.HasPrefix(topic, shadow.TopicUserThingsPrefix) {
				slog.Debug("Legacy protocol topic rejected in simple mode", "thingId", username, "topic", topic)
				return false
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
