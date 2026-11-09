package shadow

import (
	"context"
	"log/slog"
	"time"

	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"

	"ruff.io/tio/pkg/model"
)

type Operation int

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
	client   connector.PubSub
	codec    codec.Codec
	profiles connector.DeviceProfileGetter
}

func NewShadowHandler(client connector.PubSub, c codec.Codec, profiles connector.DeviceProfileGetter) StateHandler {
	return &shadowHandler{client: client, codec: c, profiles: profiles}
}

var _ StateHandler = (*shadowHandler)(nil)

func (h *shadowHandler) ShadowGetReq(ctx context.Context) (<-chan GetReqMsg, error) {
	outCh := make(chan GetReqMsg, MsgChanCap)
	err := h.client.QueueSubscribe(ctx, TopicAllGet(), "tio-shadow", func(msg connector.Message) {
		go func() {
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				slog.Error("Got wrong topic msg topic for shadow get request")
				return
			}
			c, err := codec.ForDevice(ctx, h.codec, h.profiles, thingId, "legacy")
			if err != nil {
				slog.Debug("Skip shadow get for other protocol", "thingId", thingId, "error", err)
				return
			}
			var r GetReq
			err = c.Unmarshal(msg.Payload(), &r)
			if err != nil {
				slog.Error("Invalid message payload for shadow get request")
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
	err := h.client.QueueSubscribe(ctx, TopicAllUpdate(), "tio-shadow", func(msg connector.Message) {
		go func() {
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				slog.Error("Got wrong topic msg topic for state update request")
				return
			}
			slog.Debug("Got shadow update msg", "thingId", thingId, "payload", string(msg.Payload()))

			c, err := codec.ForDevice(ctx, h.codec, h.profiles, thingId, "legacy")
			if err != nil {
				slog.Debug("Skip shadow update for other protocol", "thingId", thingId, "error", err)
				return
			}
			var r StateReq
			err = c.Unmarshal(msg.Payload(), &r)
			if err != nil {
				slog.Error("Invalid message payload for state update request")
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

func (h *shadowHandler) resolveCodec(ctx context.Context, thingId string) (codec.Codec, bool) {
	c, err := codec.ForDevice(ctx, h.codec, h.profiles, thingId, "legacy")
	if err != nil {
		if !errors.Is(err, codec.ErrProtocolMismatch) {
			slog.Error("Resolve device codec", "thingId", thingId, "error", err)
		}
		return nil, false
	}
	return c, true
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
	c, ok := h.resolveCodec(ctx, resp.ThingId)
	if !ok {
		return nil
	}
	j, err := c.Marshal(resp.Resp)
	if err != nil {
		return err
	}
	err = h.client.PublishReliable(topic, j)
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
	c, ok := h.resolveCodec(ctx, resp.ThingId)
	if !ok {
		return nil
	}
	j, err := c.Marshal(resp.Resp)
	if err != nil {
		return err
	}
	err = h.client.PublishReliable(topic, j)
	return err
}

func (h *shadowHandler) StateDeltaNotify(ctx context.Context, msg DeltaStateNoticeMsg) error {
	c, ok := h.resolveCodec(ctx, msg.ThingId)
	if !ok {
		return nil
	}
	topic := TopicDeltaStateOf(msg.ThingId)
	j, err := c.Marshal(msg.Notice)
	if err != nil {
		return errors.Wrapf(err, "marshal msg")
	}
	err = h.client.PublishReliable(topic, j)
	return err
}

func (h *shadowHandler) StateUpdatedNotify(ctx context.Context, msg StateUpdatedNoticeMsg) error {
	c, ok := h.resolveCodec(ctx, msg.ThingId)
	if !ok {
		return nil
	}
	topic := TopicStateUpdatedOf(msg.ThingId)

	j, err := c.Marshal(msg.Notice)
	if err != nil {
		return errors.Wrapf(err, "marshal msg")
	}
	err = h.client.PublishReliable(topic, j)
	return err
}

// DefaultShadow shadow value for new
func DefaultShadow(thingId string) Shadow {
	return Shadow{
		ThingId: thingId,
		Version: 1,
		Tags:    TagsValue{},
		State: StateDR{
			Desired:  StateValue{},
			Reported: StateValue{},
		},
		Metadata: Metadata{},

		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}
}
