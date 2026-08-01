package ntp

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/model"
)

const (
	TopicThingsPrefix = "$iothub/things/"
	TopicReqTmpl      = TopicThingsPrefix + "{thingId}/ntp/req"
	TopicReqAll       = TopicThingsPrefix + "+/ntp/req"
	TopicRespTmpl     = TopicThingsPrefix + "{thingId}/ntp/resp"
)

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

func NewNtpHandler(cl connector.PubSub, c codec.Codec) Handler {
	return &ntpHandler{cl, c}
}

type ntpHandler struct {
	client connector.PubSub
	codec  codec.Codec
}

func (h *ntpHandler) InitNtpHandler(ctx context.Context) error {
	topic := TopicReqAll
	err := h.client.QueueSubscribe(ctx, topic, "tio-ntp", func(msg connector.Message) {
		go func() {
			serverRecvTime := time.Now().UnixMilli()
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				slog.Error("Got wrong topic msg topic for ntp request", "error", err, "topic", msg.Topic())
				return
			}
			var r Req
			err = h.codec.Unmarshal(msg.Payload(), &r)
			if err != nil {
				slog.Error("Invalid message payload for ntp request", "payload", msg.Payload(), "topic", msg.Topic())
				return
			}
			serverSendTime := time.Now().UnixMilli()
			res := Resp{
				ClientSendTime: r.ClientSendTime,
				ServerRecvTime: serverRecvTime,
				ServerSendTime: serverSendTime,
			}
			j, err := h.codec.Marshal(res)
			if err != nil {
				slog.Error("Marshal ntp response", "response", res, "error", err, "topic", msg.Topic())
			}
			if err := h.client.Publish(TopicResp(thingId), j); err != nil {
				slog.Error("Ntp handler publish result error", "error", err, "topic", msg.Topic())
			}
		}()
	})
	return err
}
