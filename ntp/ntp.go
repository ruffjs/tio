package ntp

import (
	"context"
	"strings"
)

// Client publish a message `NtpReq` to server via topic `TopicReq`
// Server publish a message `NtpResp` to client via topic `TopicResp`

// Assume client receive time is clientRecvTime , the client can calculate the current time through this formula:
// calculationTime = ( serverRecvTime + serverSendTime + clientRecvTime - clientSendTime ) / 2

// timeSpendForReq = client ==> server
// timeSpendForResp = server ==> client
// When timeSpendForReq and timeSpendForResp are close, the calculation time is very accurate.

const (
	TopicThingsPrefix = "$iothub/things/"
	TopicReqTmpl      = TopicThingsPrefix + "{thingId}/ntp/req"
	TopicReqAll       = TopicThingsPrefix + "+/ntp/req"
	TopicRespTmpl     = TopicThingsPrefix + "{thingId}/ntp/resp"
)

// Resp All time is unix time in ms
type Resp struct {
	ClientSendTime int64 `json:"clientSendTime"`
	ServerRecvTime int64 `json:"serverRecvTime"`
	ServerSendTime int64 `json:"serverSendTime"`
}

type Req struct {
	ClientSendTime int64 `json:"clientSendTime"`
}

type Handler interface {
	InitNtpHandler(ctx context.Context) error
}

func TopicResp(thingId string) string {
	return strings.Replace(TopicRespTmpl, "{thingId}", thingId, -1)
}

func TopicReq(thingId string) string {
	return strings.Replace(TopicReqTmpl, "{thingId}", thingId, -1)
}
