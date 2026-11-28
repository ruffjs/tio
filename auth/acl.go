package auth

import (
	"context"
	"log/slog"
	"strings"

	"ruff.io/tio/config"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
)

type AclFn = func(clientId, username string, topic string, write bool) bool

// TODO Optimize: Prevent device connection if it has exceeded the maximum number of allowed operations without an Access Control List (ACL)
func TopicAcl(bg connector.BindingGetter, superUsers []config.UserPassword, protocolMode string) AclFn {
	return func(clientId, username string, topic string, write bool) bool {
		// Inline (server-side) clients and empty usernames are allow-all.
		if username == "" {
			slog.Warn("Unexpected username is empty", "clientId", clientId, "topic", topic, "write", write)
			return true
		}
		for _, u := range superUsers {
			if u.Name == username {
				return true
			}
		}

		pubAllow, pubDeny, subAllow, subDeny := thingTopicPermissions(protocolMode, username)
		if bg != nil {
			boundIds, err := bg.GetBoundThingIds(context.Background(), username)
			if err != nil {
				slog.Error("get bound things for gateway ACL", "gateway", username, "error", err)
			} else {
				for _, bid := range boundIds {
					pa, pd, sa, sd := thingTopicPermissions(protocolMode, bid)
					pubAllow = append(pubAllow, pa...)
					pubDeny = append(pubDeny, pd...)
					subAllow = append(subAllow, sa...)
					subDeny = append(subDeny, sd...)
				}
			}
		}

		op := aclOperation(write)
		if write && !filterAllowed(topic, pubAllow, pubDeny) {
			slog.Warn("MQTT ACL denied", "reason", "not in publish allow-list", "clientId", clientId, "username", username, "topic", topic, "operation", op)
			return false
		}
		if !write && !filterAllowed(topic, subAllow, subDeny) {
			slog.Warn("MQTT ACL denied", "reason", "not in subscribe allow-list", "clientId", clientId, "username", username, "topic", topic, "operation", op)
			return false
		}
		return true
	}
}

// thingTopicPermissions returns the device topics for one thing in the given
// mode, mirroring the NATS branch permission matrix. Dual mode is the union of
// legacy and simple topics.
func thingTopicPermissions(mode, thingId string) (pubAllow, pubDeny, subAllow, subDeny []string) {
	if mode == "simple" || mode == "dual" {
		prefix := protocol.TopicSimplePrefix + thingId
		pubAllow = append(pubAllow, prefix+"/#")
		pubDeny = append(pubDeny, prefix+"/"+protocol.LevelDown+"/#")
		subAllow = append(subAllow, prefix+"/#")
		subDeny = append(subDeny, prefix+"/"+protocol.LevelUp+"/#")
	}
	if mode == "legacy" || mode == "dual" {
		thingPrefix := shadow.TopicThingsPrefix + thingId
		userPrefix := shadow.TopicUserThingsPrefix + thingId
		pubAllow = append(pubAllow, thingPrefix+"/#", userPrefix+"/#")
		subAllow = append(subAllow, thingPrefix+"/#", userPrefix+"/#",
			connector.TopicPresenceEvent(thingId))
	}
	return
}

// filterAllowed reports whether filter is covered by an allow entry and not by
// any deny entry. A `+`/`#` in the filter only matches an entry with a wildcard
// at the same level, so a root `#` or a wildcard at the thingId position never
// matches a concrete allow prefix.
func filterAllowed(filter string, allow, deny []string) bool {
	for _, d := range deny {
		if filterMatches(filter, d) {
			return false
		}
	}
	for _, a := range allow {
		if filterMatches(filter, a) {
			return true
		}
	}
	return false
}

// filterMatches reports whether every topic matched by filter is also matched
// by pattern. Entries are concrete prefixes with an optional trailing `/#`.
func filterMatches(filter, pattern string) bool {
	fp := strings.Split(filter, "/")
	pp := strings.Split(pattern, "/")
	for i := 0; i < len(pp); i++ {
		if pp[i] == "#" {
			return true
		}
		if i >= len(fp) || fp[i] == "+" || fp[i] == "#" || fp[i] != pp[i] {
			return false
		}
	}
	return len(fp) == len(pp)
}

func aclOperation(write bool) string {
	if write {
		return "publish"
	}
	return "subscribe"
}
