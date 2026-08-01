package protocol

import (
	"context"
	"log/slog"
	"maps"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

const (
	maxSimpleWorkerCount = 500
)

type SimpleHandler struct {
	connector    connector.Connector
	codec        codec.Codec
	shadowSvc    shadow.Service
	pool         *ants.PoolWithFunc
	stopOnce     sync.Once
	pendingMu    sync.Mutex
	pendingCalls map[string]map[string]chan ControlMessage
}

func NewSimpleHandler(conn connector.Connector, c codec.Codec, shadowSvc shadow.Service) (*SimpleHandler, error) {
	h := &SimpleHandler{
		connector:    conn,
		codec:        c,
		shadowSvc:    shadowSvc,
		pendingCalls: make(map[string]map[string]chan ControlMessage),
	}

	pool, err := ants.NewPoolWithFunc(maxSimpleWorkerCount, func(req any) {
		r := req.(simpleRequest)
		h.handleRequest(r.ctx, r.thingId, r.msg)
	})
	if err != nil {
		return nil, errors.Wrap(err, "create worker pool")
	}
	h.pool = pool

	return h, nil
}

type simpleRequest struct {
	ctx     context.Context
	thingId string
	msg     ControlMessage
}

func (h *SimpleHandler) Start(ctx context.Context) error {
	err := h.connector.QueueSubscribe(ctx, TopicAllUp(), "tio-simple", func(msg connector.Message) {
		thingId, level, err := ParseTopic(msg.Topic())
		if err != nil {
			slog.Error("parse simple topic", "error", err, "topic", msg.Topic())
			return
		}
		if level != LevelUp {
			slog.Error("unexpected topic level", "level", level, "topic", msg.Topic())
			return
		}

		var ctrl ControlMessage
		if err := h.codec.Unmarshal(msg.Payload(), &ctrl); err != nil {
			slog.Error("unmarshal control message", "error", err, "topic", msg.Topic())
			return
		}

		switch ctrl.Type {
		case MsgTypeReply:
			h.handleReply(thingId, ctrl)
			return
		case MsgTypeReport:
			if err := validateReport(ctrl); err != nil {
				slog.Error("invalid report message", "error", err, "thingId", thingId)
				return
			}
		case MsgTypeGet:
		default:
			slog.Error("unknown message type", "type", ctrl.Type, "thingId", thingId)
			return
		}

		if err := h.pool.Invoke(simpleRequest{ctx: ctx, thingId: thingId, msg: ctrl}); err != nil {
			slog.Error("invoke worker", "error", err, "thingId", thingId)
		}
	})
	if err != nil {
		return errors.Wrap(err, "subscribe to simple up topic")
	}

	h.shadowSvc.SubscribeUpdate(h.handleShadowUpdate)
	go func() {
		<-ctx.Done()
		h.Stop()
	}()

	slog.Info("Simple protocol handler started")
	return nil
}

func (h *SimpleHandler) handleRequest(ctx context.Context, thingId string, msg ControlMessage) {
	switch msg.Type {
	case MsgTypeReport:
		h.handleReport(ctx, thingId, msg)
	case MsgTypeGet:
		h.handleGet(ctx, thingId, msg)
	default:
		slog.Error("unknown message type", "type", msg.Type, "thingId", thingId)
	}
}

func (h *SimpleHandler) handleReport(ctx context.Context, thingId string, msg ControlMessage) {
	data := msg.Data.(map[string]any)
	state := data["state"].(map[string]any)

	req := shadow.StateReq{
		State:       shadow.StateDR{Reported: state},
		ClientToken: msg.ID,
		Version:     0,
	}

	_, err := h.shadowSvc.SetReported(ctx, thingId, req)
	if err != nil {
		slog.Error("set reported", "error", err, "thingId", thingId)
	}
}

func validateReport(msg ControlMessage) error {
	data, ok := msg.Data.(map[string]any)
	if !ok {
		return errors.New("report data must be object")
	}
	if _, ok := data["state"].(map[string]any); !ok {
		return errors.New("report state must be object")
	}
	if !isNonNegativeInteger(data["version"]) {
		return errors.New("report version must be a non-negative integer")
	}
	return nil
}

func isNonNegativeInteger(value any) bool {
	switch n := value.(type) {
	case int:
		return n >= 0
	case int8:
		return n >= 0
	case int16:
		return n >= 0
	case int32:
		return n >= 0
	case int64:
		return n >= 0
	case uint, uint8, uint16, uint32:
		return true
	case uint64:
		return n <= math.MaxInt64
	case float32:
		return n >= 0 && n < float32(math.MaxInt64) && float32(math.Trunc(float64(n))) == n
	case float64:
		return n >= 0 && n < float64(math.MaxInt64) && math.Trunc(n) == n
	default:
		return false
	}
}

func (h *SimpleHandler) handleGet(ctx context.Context, thingId string, msg ControlMessage) {
	ss, err := h.shadowSvc.Get(ctx, thingId)
	if err != nil {
		slog.Error("get shadow", "error", err, "thingId", thingId)
		return
	}

	setMsg := ControlMessage{
		Type: MsgTypeSet,
		ID:   msg.ID,
		Data: map[string]any{
			"version": ss.Version,
			"state":   ss.State.Desired,
		},
	}

	payload, err := h.codec.Marshal(setMsg)
	if err != nil {
		slog.Error("marshal set message", "error", err, "thingId", thingId)
		return
	}

	if err := h.connector.PublishReliable(TopicDown(thingId), payload); err != nil {
		slog.Error("publish set message", "error", err, "thingId", thingId)
	}
}

func (h *SimpleHandler) handleReply(thingId string, msg ControlMessage) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	thingPending, ok := h.pendingCalls[thingId]
	if !ok {
		slog.Warn("reply for unknown thing", "thingId", thingId, "id", msg.ID)
		return
	}

	ch, ok := thingPending[msg.ID]
	if !ok {
		slog.Warn("reply for unknown call", "thingId", thingId, "id", msg.ID)
		return
	}

	select {
	case ch <- msg:
	default:
	}
}

func (h *SimpleHandler) handleShadowUpdate(thingId string, notice shadow.StateUpdatedNotice) {
	if shadow.IsStateValueEmpty(notice.Current.State.Desired) {
		return
	}

	delta, _ := shadow.DeltaState(notice.Current.State.Desired, notice.Current.State.Reported, nil)
	if shadow.IsStateValueEmpty(delta) {
		return
	}

	if !previousDesiredDiffers(notice.Previous.State.Desired, notice.Current.State.Desired) {
		return
	}

	setMsg := ControlMessage{
		Type: MsgTypeSet,
		Data: map[string]any{
			"version": notice.Current.Version,
			"state":   notice.Current.State.Desired,
		},
	}

	payload, err := h.codec.Marshal(setMsg)
	if err != nil {
		slog.Error("marshal set message", "error", err, "thingId", thingId)
		return
	}

	if err := h.connector.PublishReliable(TopicDown(thingId), payload); err != nil {
		slog.Error("publish set message", "error", err, "thingId", thingId)
	}
}

func previousDesiredDiffers(prev, curr map[string]any) bool {
	return !reflect.DeepEqual(prev, curr)
}

func (h *SimpleHandler) Invoke(ctx context.Context, thingId string, method string, params any, timeout time.Duration) (any, error) {
	online, err := h.connector.IsConnected(thingId)
	if err != nil {
		return nil, errors.Wrap(err, "check connection")
	}
	if !online {
		return nil, model.ErrDirectMethodThingOffline
	}

	callID, err := uuid.NewV4()
	if err != nil {
		return nil, errors.Wrap(err, "generate call ID")
	}
	callIDStr := callID.String()

	callData := map[string]any{}
	if params != nil {
		if paramsMap, ok := params.(map[string]any); ok {
			maps.Copy(callData, paramsMap)
		} else {
			callData["p"] = params
		}
	}
	callData["m"] = method

	callMsg := ControlMessage{
		Type: MsgTypeCall,
		ID:   callIDStr,
		Data: callData,
	}

	payload, err := h.codec.Marshal(callMsg)
	if err != nil {
		return nil, errors.Wrap(err, "marshal call message")
	}

	replyCh := make(chan ControlMessage, 1)
	h.addPendingCall(thingId, callIDStr, replyCh)
	defer h.removePendingCall(thingId, callIDStr)

	if err := h.connector.PublishReliable(TopicDown(thingId), payload); err != nil {
		return nil, errors.Wrap(err, "publish call message")
	}

	select {
	case <-time.After(timeout):
		return nil, model.ErrDirectMethodTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	case reply := <-replyCh:
		return reply.Data, nil
	}
}

func (h *SimpleHandler) addPendingCall(thingId, callID string, ch chan ControlMessage) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	if h.pendingCalls[thingId] == nil {
		h.pendingCalls[thingId] = make(map[string]chan ControlMessage)
	}
	h.pendingCalls[thingId][callID] = ch
}

func (h *SimpleHandler) removePendingCall(thingId, callID string) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	if thingPending, ok := h.pendingCalls[thingId]; ok {
		delete(thingPending, callID)
		if len(thingPending) == 0 {
			delete(h.pendingCalls, thingId)
		}
	}
}

func (h *SimpleHandler) Stop() {
	h.stopOnce.Do(func() {
		if h.pool != nil {
			h.pool.Release()
		}
	})
}
