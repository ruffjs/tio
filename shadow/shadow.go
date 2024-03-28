package shadow

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"ruff.io/tio/connector"

	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
)

type Operation int

const DefaultQos = 1
const (
	OpGet Operation = iota
	OpUpdate
)

type StateHandler interface {
	// ShadowGetReq the request to get device shadow
	ShadowGetReq(ctx context.Context) (<-chan GetReqMsg, error)
	// StateUpdateReq either update state desired or reported, not both the same time
	// State.Desired or State.Desired should be nil
	StateUpdateReq(ctx context.Context) (<-chan StateReqMsg, error)
	RejectedResp(ctx context.Context, resp ErrRespMsg) error
	AcceptedResp(ctx context.Context, resp StateAcceptedRespMsg) error
	StateDeltaNotify(ctx context.Context, notice DeltaStateNoticeMsg) error
	StateUpdatedNotify(ctx context.Context, notice StateUpdatedNoticeMsg) error
}

type GetReqMsg struct {
	ThingId string
	Req     GetReq
}

type StateReqMsg struct {
	ThingId string
	Req     StateReq
}

type ErrRespMsg struct {
	ThingId string
	Op      Operation
	Resp    ErrResp
}

type StateAcceptedRespMsg struct {
	ThingId string
	Op      Operation
	Resp    StateAcceptedResp
}

type DeltaStateNoticeMsg struct {
	ThingId string
	Notice  DeltaStateNotice
}

type StateUpdatedNoticeMsg struct {
	ThingId string
	Notice  StateUpdatedNotice
}

// implement

const (
	MsgChanCap = 10000
)

type shadowHandler struct {
	client connector.PubSub
}

func NewShadowHandler(client connector.PubSub) StateHandler {
	return &shadowHandler{client}
}

var _ StateHandler = (*shadowHandler)(nil)

func (h *shadowHandler) ShadowGetReq(ctx context.Context) (<-chan GetReqMsg, error) {
	outCh := make(chan GetReqMsg, MsgChanCap)
	err := h.client.Subscribe(ctx, TopicAllGet(), DefaultQos, func(msg connector.Message) {
		go func() {
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for shadow get request")
				return
			}
			var r GetReq
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for shadow get request")
				return
			}
			res := GetReqMsg{
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

func (h *shadowHandler) StateUpdateReq(ctx context.Context) (<-chan StateReqMsg, error) {
	outCh := make(chan StateReqMsg, MsgChanCap)
	err := h.client.Subscribe(ctx, TopicAllUpdate(), DefaultQos, func(msg connector.Message) {
		go func() {
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for state update request")
				return
			}
			var r StateReq
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for state update request")
				return
			}
			res := StateReqMsg{
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

func (h *shadowHandler) RejectedResp(ctx context.Context, resp ErrRespMsg) error {
	topic := ""
	switch resp.Op {
	case OpGet:
		topic = TopicGetRejectedOf(resp.ThingId)
	case OpUpdate:
		topic = TopicUpdateRejectedOf(resp.ThingId)
	default:
		return errors.Errorf("unsupported shadow operation %d", resp.Op)
	}
	j, err := json.Marshal(resp.Resp)
	if err != nil {
		return err
	}
	err = h.client.Publish(topic, DefaultQos, false, j)
	return err
}

func (h *shadowHandler) AcceptedResp(ctx context.Context, resp StateAcceptedRespMsg) error {
	topic := ""
	switch resp.Op {
	case OpGet:
		topic = TopicGetAcceptedOf(resp.ThingId)
	case OpUpdate:
		topic = TopicUpdateAcceptedOf(resp.ThingId)
	default:
		return errors.Errorf("unsupported shadow operation %d", resp.Op)
	}
	j, err := json.Marshal(resp.Resp)
	if err != nil {
		return err
	}
	err = h.client.Publish(topic, DefaultQos, false, j)
	return err
}

func (h *shadowHandler) StateDeltaNotify(ctx context.Context, msg DeltaStateNoticeMsg) error {
	topic := TopicDeltaStateOf(msg.ThingId)
	j, err := json.Marshal(msg.Notice)
	if err != nil {
		return errors.Wrapf(err, "marshal msg")
	}
	err = h.client.Publish(topic, DefaultQos, false, j)
	return err
}

func (h *shadowHandler) StateUpdatedNotify(ctx context.Context, msg StateUpdatedNoticeMsg) error {
	topic := TopicStateUpdatedOf(msg.ThingId)

	j, err := json.Marshal(msg.Notice)
	if err != nil {
		return errors.Wrapf(err, "marshal msg")
	}
	err = h.client.Publish(topic, DefaultQos, false, j)
	return err
}
