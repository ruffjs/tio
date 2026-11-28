package embed

// forward whitelist: decide which device publishes are bridged out (NATS).

import (
	"strings"

	"ruff.io/tio/pkg/protocol"
	"ruff.io/tio/shadow"
)

// legacyReserved are the control-plane paths under $iothub/things/{id}/ that
// must never be forwarded. A trailing slash means "this path and everything
// beneath it"; otherwise only the exact path is reserved.
var legacyReserved = []string{
	"shadows/",
	"methods/",
	"ntp/",
	"jobs", "jobs/",
	"presence",
	"will", "will/",
	"messages/will", "messages/will/",
}

// forwardable reports whether a device publish to topic should be forwarded to
// the bridge under the given protocol mode. It is only consulted for non-inline
// (device-session) publishes; ACL has already authorized the topic.
func forwardable(mode, topic string) bool {
	switch mode {
	case "legacy":
		return forwardableLegacy(topic)
	case "simple":
		return forwardableSimple(topic)
	case "dual":
		return forwardableLegacy(topic) || forwardableSimple(topic)
	default:
		return false
	}
}

func forwardableLegacy(topic string) bool {
	if sub, ok := thingSub(topic, shadow.TopicUserThingsPrefix); ok && sub != "" {
		return true // $iothub/user/things/{id}/# is always forwarded
	}
	if sub, ok := thingSub(topic, shadow.TopicThingsPrefix); ok && sub != "" {
		return !isLegacyReserved(sub)
	}
	return false
}

func forwardableSimple(topic string) bool {
	thingId, dir, err := protocol.ParseTopicDir(topic)
	if err != nil {
		return false
	}
	switch dir {
	case protocol.LevelEvent, protocol.LevelData, "custom":
		return true
	case protocol.LevelDown:
		return false
	case protocol.LevelUp:
		return !isSimpleControlType(upType(topic, thingId))
	default:
		return false
	}
}

// thingSub returns the path after "{prefix}{thingId}/", or ok=false if topic
// does not match the prefix with a thingId and a sub-path.
func thingSub(topic, prefix string) (string, bool) {
	if !strings.HasPrefix(topic, prefix) {
		return "", false
	}
	rest := topic[len(prefix):]
	idx := strings.Index(rest, "/")
	if idx < 0 {
		return "", false
	}
	return rest[idx+1:], true
}

func isLegacyReserved(sub string) bool {
	for _, p := range legacyReserved {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(sub, p) {
				return true
			}
		} else if sub == p {
			return true
		}
	}
	return false
}

// upType returns the message type in a tio/{thingId}/up/{type} topic, empty if
// the topic has no type level.
func upType(topic, thingId string) string {
	prefix := protocol.TopicUp(thingId, "")
	if !strings.HasPrefix(topic, prefix) {
		return ""
	}
	typ := topic[len(prefix):]
	if i := strings.Index(typ, "/"); i >= 0 {
		typ = typ[:i]
	}
	return typ
}

// isSimpleControlType reports whether typ is a control-plane uplink type that
// must not be forwarded (spec §6.4).
func isSimpleControlType(typ string) bool {
	switch typ {
	case protocol.TypeShadowGet, protocol.TypeShadowUpdate, protocol.TypeMethodResp, protocol.TypeNtpReq:
		return true
	}
	return false
}
