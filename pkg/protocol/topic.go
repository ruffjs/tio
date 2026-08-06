package protocol

import (
	"fmt"
	"strings"
)

const (
	TopicSimplePrefix = "tio/"

	LevelUp    = "up"
	LevelDown  = "down"
	LevelEvent = "event"
	LevelData  = "data"
)

// Control message types (used in topic path: tio/{thingId}/{up|down}/{type})
const (
	TypeShadowGet         = "shadow_get"
	TypeShadowGetReply    = "shadow_get_reply"
	TypeShadowUpdate      = "shadow_update"
	TypeShadowUpdateReply = "shadow_update_reply"
	TypeShadowDesired     = "shadow_desired"
	TypeMethodReq         = "method_req"
	TypeMethodResp        = "method_resp"
	TypeNtpReq            = "ntp_req"
	TypeNtpResp           = "ntp_resp"
)

func TopicUp(thingId, typ string) string {
	return TopicSimplePrefix + thingId + "/" + LevelUp + "/" + typ
}

func TopicDown(thingId, typ string) string {
	return TopicSimplePrefix + thingId + "/" + LevelDown + "/" + typ
}

func TopicEvent(thingId string) string {
	return TopicSimplePrefix + thingId + "/" + LevelEvent
}

func TopicData(thingId string) string {
	return TopicSimplePrefix + thingId + "/" + LevelData
}

// TopicAllUp returns the MQTT subscription filter for all up messages.
// Matches tio/{thingId}/up/{type} and deeper.
func TopicAllUp() string {
	return TopicSimplePrefix + "+/" + LevelUp + "/#"
}

// ParseTopic parses a simple protocol topic.
// Returns thingId, direction (up/down/event/data), typ (message type, empty for event/data), err.
func ParseTopic(topic string) (thingId, direction, typ string, err error) {
	if !strings.HasPrefix(topic, TopicSimplePrefix) {
		return "", "", "", fmt.Errorf("invalid topic prefix: %s", topic)
	}
	rest := topic[len(TopicSimplePrefix):]
	parts := strings.Split(rest, "/")

	switch len(parts) {
	case 2:
		// tio/{thingId}/{event|data}
		thingId = parts[0]
		direction = parts[1]
		if direction != LevelEvent && direction != LevelData {
			return "", "", "", fmt.Errorf("invalid topic level: %s", topic)
		}
	case 3:
		// tio/{thingId}/{up|down}/{type}
		thingId = parts[0]
		direction = parts[1]
		typ = parts[2]
		if direction != LevelUp && direction != LevelDown {
			return "", "", "", fmt.Errorf("invalid topic direction: %s", topic)
		}
		if typ == "" {
			return "", "", "", fmt.Errorf("empty type in topic: %s", topic)
		}
	default:
		return "", "", "", fmt.Errorf("invalid topic structure: %s", topic)
	}

	if thingId == "" || thingId == "+" || thingId == "#" {
		return "", "", "", fmt.Errorf("invalid thingId in topic: %s", topic)
	}

	return thingId, direction, typ, nil
}

// ParseTopicDir leniently extracts thingId and the first segment after it
// from any tio/{thingId}/... topic. Unlike ParseTopic, it accepts unknown
// directions and arbitrary sub-paths, for use in ACL checks where custom
// topics under tio/{thingId}/# should be allowed.
func ParseTopicDir(topic string) (thingId, direction string, err error) {
	if !strings.HasPrefix(topic, TopicSimplePrefix) {
		return "", "", fmt.Errorf("invalid topic prefix: %s", topic)
	}
	rest := topic[len(TopicSimplePrefix):]
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid topic structure: %s", topic)
	}
	thingId = parts[0]
	direction = parts[1]
	if idx := strings.Index(direction, "/"); idx >= 0 {
		direction = direction[:idx]
	}
	if thingId == "+" || thingId == "#" {
		return "", "", fmt.Errorf("invalid thingId in topic: %s", topic)
	}
	return thingId, direction, nil
}
