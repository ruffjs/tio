package mqtt

import (
	"context"
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"ruff.io/tio/connector/mqtt/client"
	"ruff.io/tio/ntp"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

func NewNtpHandler(cl client.Client) ntp.Handler {
	return &ntpHandler{cl}
}

type ntpHandler struct {
	client client.Client
}

func (h *ntpHandler) InitNtpHandler(ctx context.Context) error {
	topic := ntp.TopicReqAll
	err := h.client.Subscribe(ctx, topic, DefaultQos, func(client mqtt.Client, msg mqtt.Message) {
		go func() {
			serverRecvTime := time.Now().UnixMilli()
			thingId, err := shadow.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for ntp request: %s", err)
				return
			}
			var r ntp.Req
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for ntp request: %s", msg.Payload())
				return
			}
			serverSendTime := time.Now().UnixMilli()
			res := ntp.Resp{
				ClientSendTime: r.ClientSendTime,
				ServerRecvTime: serverRecvTime,
				ServerSendTime: serverSendTime,
			}
			j, err := json.Marshal(res)
			if err != nil {
				log.Errorf("Marshal ntp response %#v error: %s", res, err)
			}
			h.client.Publish(ntp.TopicResp(thingId), 0, false, j)
		}()
	})
	return err
}
