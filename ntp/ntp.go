package ntp

import (
	"context"
	"strings"
	"time"

	"ruff.io/tio/connector"

	"encoding/json"

	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
)

// Client publish a message `NtpReq` to server via topic `TopicReq`
// Server publish a message `NtpResp` to client via topic `TopicResp`

// Assume client receive time is clientRecvTime , the client can calculate the current time through this formula:
// calculationTime = ( serverRecvTime + serverSendTime + clientRecvTime - clientSendTime ) / 2

// timeSpendForReq = client ==> server
// timeSpendForResp = server ==> client
// When timeSpendForReq and timeSpendForResp are close, the calculation time is very accurate.

const DefaultQos = 1

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

func NewNtpHandler(cl connector.PubSub) Handler {
	return &ntpHandler{cl}
}

type ntpHandler struct {
	client connector.PubSub
}

func (h *ntpHandler) InitNtpHandler(ctx context.Context) error {
	topic := TopicReqAll
	err := h.client.Subscribe(ctx, topic, DefaultQos, func(msg connector.Message) {
		go func() {
			serverRecvTime := time.Now().UnixMilli()
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for ntp request: %s, topic=%q", err, msg.Topic())
				return
			}
			var r Req
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for ntp request: %s, topic=%q", msg.Payload(), msg.Topic())
				return
			}
			serverSendTime := time.Now().UnixMilli()
			res := Resp{
				ClientSendTime: r.ClientSendTime,
				ServerRecvTime: serverRecvTime,
				ServerSendTime: serverSendTime,
			}
			j, err := json.Marshal(res)
			if err != nil {
				log.Errorf("Marshal ntp response %#v error: %s, topic=%q", res, err, msg.Topic())
			}
			if err := h.client.Publish(TopicResp(thingId), 0, false, j); err != nil {
				log.Errorf("Ntp handler publish result error: %v, topic=%q", err, msg.Topic())
			}
		}()
	})
	return err
}
