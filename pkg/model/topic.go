package model

import (
	"fmt"
	"regexp"
)

// Topic thingId must after string "things/"
var reg4topicThingId = regexp.MustCompile(`things/([0-9a-zA-Z_-]+)`)

func GetThingIdFromTopic(topic string) (string, error) {
	match := reg4topicThingId.FindStringSubmatch(topic)
	if len(match) == 2 {
		return match[1], nil
	}
	return "", fmt.Errorf("no thingId in topic")
}
