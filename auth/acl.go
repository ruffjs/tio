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
				slog.Warn("MQTT ACL denied", "reason", "simple topic in legacy mode", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write))
				return false
			}

			thingId, direction, err := protocol.ParseTopicDir(topic)
			if err != nil {
				slog.Warn("MQTT ACL denied", "reason", "invalid simple topic", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write), "error", err)
				return false
			}

			// Check if thing matches username or is bound to gateway
			if thingId != username {
				if bound, err := bg.IsBoundGateway(context.Background(), thingId, username); err != nil || !bound {
					slog.Warn("MQTT ACL denied", "reason", "topic belongs to another thing", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write), "targetThingId", thingId, "error", err)
					return false
				}
			}

			// Deny-list: only restrict known directions, allow everything else
			// under tio/{thingId}/# for free messaging via NATS.
			if write {
				if direction == protocol.LevelDown {
					slog.Warn("MQTT ACL denied", "reason", "device cannot publish to down topic", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write))
					return false
				}
			} else {
				if direction == protocol.LevelUp {
					slog.Warn("MQTT ACL denied", "reason", "device cannot subscribe to up topic", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write))
					return false
				}
			}

			return true
		}

		// Legacy protocol topics
		if protocolMode == "simple" {
			// Reject legacy topics in simple mode
			if strings.HasPrefix(topic, shadow.TopicThingsPrefix) || strings.HasPrefix(topic, shadow.TopicUserThingsPrefix) {
				slog.Warn("MQTT ACL denied", "reason", "legacy topic in simple mode", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write))
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
				slog.Warn("MQTT ACL denied", "reason", "cannot parse target thing id", "clientId", clientId, "username", username, "topic", topic, "operation", aclOperation(write), "error", err)
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
			slog.Warn("MQTT ACL denied", "reason", "target thing is not bound to gateway", "clientId", clientId, "username", username, "topic", topic, "operation", op, "targetThingId", tingId, "error", err)
			return false
		} else {
			// Non-reserved topics
			return true
		}
	}
}

func aclOperation(write bool) string {
	if write {
		return "publish"
	}
	return "subscribe"
}
