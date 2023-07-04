// shadow  message definitions
// reference to aws iot

package shadow

import (
	"strings"

	"github.com/pkg/errors"
)

// topics
const (
	TopicThingsPrefix     = "$iothub/things/"
	TopicUserThingsPrefix = "$iothub/user/things/"

	// TopicPrefixTmpl thing topic prefix
	TopicPrefixTmpl = TopicThingsPrefix + "{thingId}/shadows/name/default"
	TopicPrefixAll  = TopicThingsPrefix + "+/shadows/name/default"

	// TopicGet Publish an empty message to this topic to get the device's shadow
	TopicGet = "/get"
	// TopicGetAccepted tio publishes a response shadow document to this topic when returning the device's shadow
	// message StateAcceptedResp
	TopicGetAccepted = "/get/accepted"
	// TopicGetRejected tio publishes a response shadow document to this topic when returning the device's shadow
	// message ErrResp
	TopicGetRejected = "/get/rejected"

	// TopicUpdate Publish a request state document to this topic to update the device's shadow
	// The message body contains a partial request state document.
	TopicUpdate = "/update"
	// TopicUpdateAccepted message StateAcceptedResp
	TopicUpdateAccepted = "/update/accepted"
	// TopicUpdateRejected message ErrResp
	TopicUpdateRejected = "/update/rejected"
	// TopicUpdateDelta tio publishes a response state document to this topic
	// when it accepts a change for the device's shadow,
	// and the request state document contains different values for desired and reported states
	// message DeltaStateNotice
	TopicUpdateDelta = "/update/delta"
	// TopicUpdateDocuments tio publishes a state document to this topic whenever an update to the shadow is successfully performed:
	// message StateUpdatedNotice
	TopicUpdateDocuments = "/update/documents"
)

// functions for get topic

func TopicAllGet() string {
	return TopicPrefixAll + TopicGet
}

func TopicAllUpdate() string {
	return TopicPrefixAll + TopicUpdate
}

func TopicUpdateOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicUpdate
}

func TopicGetAcceptedOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicGetAccepted
}

func TopicGetRejectedOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicGetRejected
}

func TopicUpdateAcceptedOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicUpdateAccepted
}

func TopicUpdateRejectedOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicUpdateRejected
}

func TopicStateUpdatedOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicUpdateDocuments
}

func TopicDeltaStateOf(thingId string) string {
	return topicThingPrefixOf(thingId) + TopicUpdateDelta
}

func topicThingPrefixOf(thingId string) string {
	return strings.Replace(TopicPrefixTmpl, "{thingId}", thingId, -1)
}

func GetThingIdFromTopic(topic string) (string, error) {
	arr := strings.Split(topic, "/")
	if len(arr) < 4 {
		return "", errors.Errorf("topic name is invalid %s", topic)
	}
	if arr[0] != "$iothub" || arr[1] != "things" {
		return "", errors.Errorf("topic name is invalid %s", topic)
	}
	return arr[2], nil
}
