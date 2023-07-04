package mqtt

import (
	"context"
	"encoding/json"
	"time"

	mqc "ruff.io/tio/connector/mqtt/client"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/shadow"
)

const (
	MsgChanCap  = 10000
	SendTimeout = time.Second * 3
)

type shadowHandler struct {
	client mqc.Client
}

func NewShadowHandler(client mqc.Client) shadow.StateHandler {
	return &shadowHandler{client}
}

var _ shadow.StateHandler = (*shadowHandler)(nil)

func (h *shadowHandler) ShadowGetReq(ctx context.Context) (<-chan shadow.GetReqMsg, error) {
	outCh := make(chan shadow.GetReqMsg, MsgChanCap)
	err := h.client.Subscribe(ctx, shadow.TopicAllGet(), DefaultQos, func(client mqtt.Client, msg mqtt.Message) {
		go func() {
			thingId, err := shadow.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for shadow get request")
				return
			}
			var r shadow.GetReq
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for shadow get request")
				return
			}
			res := shadow.GetReqMsg{
				ThingId: thingId,
				Req:     r,
			}
			select {
			case <-ctx.Done():
			case outCh <- res:
			}
		}()
	})
	if err != nil {
		return nil, err
	}
	return outCh, nil
}

func (h *shadowHandler) StateUpdateReq(ctx context.Context) (<-chan shadow.StateReqMsg, error) {
	outCh := make(chan shadow.StateReqMsg, MsgChanCap)
	err := h.client.Subscribe(ctx, shadow.TopicAllUpdate(), DefaultQos, func(client mqtt.Client, msg mqtt.Message) {
		go func() {
			thingId, err := shadow.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for state update request")
				return
			}
			var r shadow.StateReq
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for state update request")
				return
			}
			res := shadow.StateReqMsg{
				ThingId: thingId,
				Req:     r,
			}
			select {
			case <-ctx.Done():
			case outCh <- res:
			}
		}()
	})
	if err != nil {
		return nil, err
	}
	return outCh, nil
}

func (h *shadowHandler) RejectedResp(ctx context.Context, resp shadow.ErrRespMsg) error {
	topic := ""
	switch resp.Op {
	case shadow.OpGet:
		topic = shadow.TopicGetRejectedOf(resp.ThingId)
	case shadow.OpUpdate:
		topic = shadow.TopicUpdateRejectedOf(resp.ThingId)
	default:
		return errors.Errorf("unsupported shadow operation %d", resp.Op)
	}
	j, err := json.Marshal(resp.Resp)
	if err != nil {
		return err
	}
	token := h.client.Publish(topic, DefaultQos, false, j)
	select {
	case <-time.After(SendTimeout):
		return errors.Errorf("send timeout")
	case <-token.Done():
		return token.Error()
	}
}

func (h *shadowHandler) AcceptedResp(ctx context.Context, resp shadow.StateAcceptedRespMsg) error {
	topic := ""
	switch resp.Op {
	case shadow.OpGet:
		topic = shadow.TopicGetAcceptedOf(resp.ThingId)
	case shadow.OpUpdate:
		topic = shadow.TopicUpdateAcceptedOf(resp.ThingId)
	default:
		return errors.Errorf("unsupported shadow operation %d", resp.Op)
	}
	j, err := json.Marshal(resp.Resp)
	if err != nil {
		return err
	}
	token := h.client.Publish(topic, DefaultQos, false, j)
	select {
	case <-time.After(SendTimeout):
		return errors.Errorf("send timeout")
	case <-token.Done():
		return token.Error()
	}
}

func (h *shadowHandler) StateDeltaNotify(ctx context.Context, msg shadow.DeltaStateNoticeMsg) error {
	topic := shadow.TopicDeltaStateOf(msg.ThingId)
	j, err := json.Marshal(msg.Notice)
	if err != nil {
		return errors.Wrapf(err, "marshal msg")
	}
	token := h.client.Publish(topic, DefaultQos, false, j)
	select {
	case <-time.After(SendTimeout):
		return errors.Errorf("send timeout")
	case <-token.Done():
		return token.Error()
	}
}

func (h *shadowHandler) StateUpdatedNotify(ctx context.Context, msg shadow.StateUpdatedNoticeMsg) error {
	topic := shadow.TopicStateUpdatedOf(msg.ThingId)

	j, err := json.Marshal(msg.Notice)
	if err != nil {
		return errors.Wrapf(err, "marshal msg")
	}
	token := h.client.Publish(topic, DefaultQos, false, j)
	select {
	case <-time.After(SendTimeout):
		return errors.Errorf("send timeout")
	case <-token.Done():
		return token.Error()
	}
}
