package protocol

import (
	"fmt"
	"strings"
)

const (
	TopicSimplePrefix = "tio/"
	LevelUp           = "up"
	LevelDown         = "down"
	LevelEvent        = "event"
	LevelData         = "data"
)

func TopicUp(thingId string) string {
	return TopicSimplePrefix + thingId + "/" + LevelUp
}

func TopicDown(thingId string) string {
	return TopicSimplePrefix + thingId + "/" + LevelDown
}

func TopicEvent(thingId string) string {
	return TopicSimplePrefix + thingId + "/" + LevelEvent
}

func TopicData(thingId string) string {
	return TopicSimplePrefix + thingId + "/" + LevelData
}

func TopicAllUp() string {
	return TopicSimplePrefix + "+/" + LevelUp
}

func ParseTopic(topic string) (thingId, level string, err error) {
	if !strings.HasPrefix(topic, TopicSimplePrefix) {
		return "", "", fmt.Errorf("invalid topic prefix: %s", topic)
	}
	rest := topic[len(TopicSimplePrefix):]
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid topic structure: %s", topic)
	}
	thingId = parts[0]
	level = parts[1]
	if thingId == "" || thingId == "+" || thingId == "#" {
		return "", "", fmt.Errorf("invalid thingId in topic: %s", topic)
	}
	if level != LevelUp && level != LevelDown && level != LevelEvent && level != LevelData {
		return "", "", fmt.Errorf("invalid topic level: %s", level)
	}
	return thingId, level, nil
}
