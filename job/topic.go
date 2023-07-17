package job

import "strings"

const (
	TopicPrefixTmpl = "$iothub/things/{thingId}/jobs"

	TopicNotifyTmpl     = TopicPrefixTmpl + "/notify"
	TopicNotifyNextTmpl = TopicPrefixTmpl + "/notify-next"

	TopicGetListTmpl         = TopicPrefixTmpl + "/get"
	TopicGetListAcceptedTmpl = TopicPrefixTmpl + "/get/accepted"
	TopicGetListRejectedTmpl = TopicPrefixTmpl + "/get/rejected"

	NextJobId            = "$next"
	TopicGetTmpl         = TopicPrefixTmpl + "/{jobId}/get"
	TopicGetAcceptedTmpl = TopicPrefixTmpl + "/{jobId}/get/accepted"
	TopicGetRejectedTmpl = TopicPrefixTmpl + "/{jobId}/get/rejected"

	TopicStartNextTmpl = TopicPrefixTmpl + "/start-next"

	TopicUpdateTmpl         = TopicPrefixTmpl + "/{jobId}/update"
	TopicUpdateAcceptedTmpl = TopicPrefixTmpl + "/{jobId}/update/accepted"
	TopicUpdateRejectedTmpl = TopicPrefixTmpl + "/{jobId}/update/rejected"
)

func replaceThingId(s, thingId string) string {
	return strings.ReplaceAll(s, "{thingId}", thingId)
}
func replaceJobId(s, jobId string) string {
	return strings.ReplaceAll(s, "{jobId}", jobId)
}

func TopicNotify(thingId string) string {
	return replaceThingId(TopicNotifyTmpl, thingId)
}

func TopicNotifyNext(thingId string) string {
	return replaceThingId(TopicNotifyNextTmpl, thingId)
}

func TopicGetList(thingId string) string {
	return replaceThingId(TopicGetListTmpl, thingId)
}
func TopicGetListAccepted(thingId string) string {
	return replaceThingId(TopicGetListAcceptedTmpl, thingId)
}
func TopicGetListRejected(thingId string) string {
	return replaceThingId(TopicGetListRejectedTmpl, thingId)
}

func TopicStartNext(thingId string) string {
	return replaceThingId(TopicStartNextTmpl, thingId)
}

func TopicGet(thingId, jobId string) string {
	return replaceJobId(replaceThingId(TopicGetTmpl, thingId), jobId)
}
func TopicGetAccepted(thingId, jobId string) string {
	return replaceJobId(replaceThingId(TopicGetAcceptedTmpl, thingId), jobId)
}
func TopicGetRejected(thingId, jobId string) string {
	return replaceJobId(replaceThingId(TopicGetRejectedTmpl, thingId), jobId)
}

func TopicUpdate(thingId, jobId string) string {
	return replaceJobId(replaceThingId(TopicUpdateTmpl, thingId), jobId)
}
func TopicUpdateAccepted(thingId, jobId string) string {
	return replaceJobId(replaceThingId(TopicUpdateAcceptedTmpl, thingId), jobId)
}
func TopicUpdateRejected(thingId, jobId string) string {
	return replaceJobId(replaceThingId(TopicUpdateRejectedTmpl, thingId), jobId)
}
