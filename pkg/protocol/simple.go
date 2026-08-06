package protocol

import (
	"context"
	"log/slog"
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
	pendingCalls map[string]map[string]chan MethodResp
}

func NewSimpleHandler(conn connector.Connector, c codec.Codec, shadowSvc shadow.Service) (*SimpleHandler, error) {
	h := &SimpleHandler{
		connector:    conn,
		codec:        c,
		shadowSvc:    shadowSvc,
		pendingCalls: make(map[string]map[string]chan MethodResp),
	}

	pool, err := ants.NewPoolWithFunc(maxSimpleWorkerCount, func(req any) {
		r := req.(simpleRequest)
		h.handlePoolRequest(r.ctx, r.thingId, r.typ, r.payload)
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
	typ     string
	payload []byte
}

func (h *SimpleHandler) Start(ctx context.Context) error {
	err := h.connector.QueueSubscribe(ctx, TopicAllUp(), "tio-simple", func(msg connector.Message) {
		thingId, direction, typ, err := ParseTopic(msg.Topic())
		if err != nil {
			slog.Error("parse simple topic", "error", err, "topic", msg.Topic())
			return
		}
		if direction != LevelUp {
			slog.Error("unexpected topic direction", "direction", direction, "topic", msg.Topic())
			return
		}

		switch typ {
		case TypeMethodResp:
			h.handleMethodResp(thingId, msg.Payload())
		case TypeNtpReq:
			h.handleNtpReq(thingId, msg.Payload())
		case TypeShadowGet, TypeShadowUpdate:
			if err := h.pool.Invoke(simpleRequest{ctx: ctx, thingId: thingId, typ: typ, payload: msg.Payload()}); err != nil {
				slog.Error("invoke worker", "error", err, "thingId", thingId)
			}
		default:
			slog.Warn("unknown up type", "type", typ, "thingId", thingId)
		}
	})
	if err != nil {
		return errors.Wrap(err, "subscribe to simple up topic")
	}

	h.shadowSvc.SubscribeUpdate(h.handleShadowDesired)
	go func() {
		<-ctx.Done()
		h.Stop()
	}()

	slog.Info("Simple protocol handler started")
	return nil
}

func (h *SimpleHandler) handlePoolRequest(ctx context.Context, thingId, typ string, payload []byte) {
	switch typ {
	case TypeShadowGet:
		h.handleShadowGet(ctx, thingId)
	case TypeShadowUpdate:
		h.handleShadowUpdate(ctx, thingId, payload)
	default:
		slog.Warn("unknown pool request type", "type", typ, "thingId", thingId)
	}
}

func (h *SimpleHandler) handleShadowGet(ctx context.Context, thingId string) {
	ss, err := h.shadowSvc.Get(ctx, thingId)
	if err != nil {
		h.publish(TopicDown(thingId, TypeShadowGetReply), ShadowGetReply{
			Code:    500,
			Message: "Failed to read shadow",
		})
		return
	}

	desired := ss.State.Desired
	if desired == nil {
		desired = map[string]any{}
	}
	reported := ss.State.Reported
	if reported == nil {
		reported = map[string]any{}
	}

	h.publish(TopicDown(thingId, TypeShadowGetReply), ShadowGetReply{
		Code:    200,
		Version: ss.Version,
		State: &ShadowState{
			Desired:  desired,
			Reported: reported,
		},
	})
}

func (h *SimpleHandler) handleShadowUpdate(ctx context.Context, thingId string, payload []byte) {
	var req ShadowUpdateReq
	if err := h.codec.Unmarshal(payload, &req); err != nil {
		h.publish(TopicDown(thingId, TypeShadowUpdateReply), ShadowUpdateReply{
			Code:    400,
			Message: "Invalid shadow update",
		})
		return
	}
	if err := validateShadowUpdate(req); err != nil {
		h.publish(TopicDown(thingId, TypeShadowUpdateReply), ShadowUpdateReply{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	sr := shadow.StateReq{
		State:   shadow.StateDR{Reported: req.State},
		Version: req.Version,
	}
	ss, err := h.shadowSvc.SetReported(ctx, thingId, sr)
	if err != nil {
		code := 500
		var version int64
		if errors.Is(err, model.ErrVersionConflict) {
			code = 409
			if cur, gErr := h.shadowSvc.Get(ctx, thingId); gErr == nil {
				version = cur.Version
			}
		} else if errors.Is(err, model.ErrNotFound) {
			code = 404
		} else if errors.Is(err, model.ErrShadowFormat) || errors.Is(err, model.ErrInvalidParams) {
			code = 400
		}
		reply := ShadowUpdateReply{
			Code:    code,
			Message: err.Error(),
		}
		if code == 409 {
			reply.Version = version
		}
		h.publish(TopicDown(thingId, TypeShadowUpdateReply), reply)
		return
	}

	h.publish(TopicDown(thingId, TypeShadowUpdateReply), ShadowUpdateReply{
		Code:    200,
		Version: ss.Version,
	})
}

func validateShadowUpdate(req ShadowUpdateReq) error {
	if req.State == nil {
		return errors.New("state must be object")
	}
	return nil
}

func (h *SimpleHandler) handleNtpReq(thingId string, payload []byte) {
	serverRecvTime := time.Now().UnixMilli()

	var req NtpReq
	if err := h.codec.Unmarshal(payload, &req); err != nil {
		h.publish(TopicDown(thingId, TypeNtpResp), NtpResp{
			Code:    400,
			Message: "Invalid ntp request",
		})
		return
	}
	if req.ClientSendTime <= 0 {
		h.publish(TopicDown(thingId, TypeNtpResp), NtpResp{
			Code:    400,
			Message: "Invalid clientSendTime",
		})
		return
	}

	serverSendTime := time.Now().UnixMilli()

	h.publish(TopicDown(thingId, TypeNtpResp), NtpResp{
		Code:           200,
		ClientSendTime: req.ClientSendTime,
		ServerRecvTime: serverRecvTime,
		ServerSendTime: serverSendTime,
	})
}

func (h *SimpleHandler) handleMethodResp(thingId string, payload []byte) {
	var resp MethodResp
	if err := h.codec.Unmarshal(payload, &resp); err != nil {
		slog.Error("unmarshal method resp", "error", err, "thingId", thingId)
		return
	}
	if resp.ID == "" {
		slog.Warn("method resp missing id", "thingId", thingId)
		return
	}

	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	thingPending, ok := h.pendingCalls[thingId]
	if !ok {
		slog.Warn("method resp for unknown thing", "thingId", thingId, "id", resp.ID)
		return
	}

	ch, ok := thingPending[resp.ID]
	if !ok {
		slog.Warn("method resp for unknown call", "thingId", thingId, "id", resp.ID)
		return
	}

	select {
	case ch <- resp:
	default:
	}
}

func (h *SimpleHandler) handleShadowDesired(thingId string, notice shadow.StateUpdatedNotice) {
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

	h.publish(TopicDown(thingId, TypeShadowDesired), ShadowDesired{
		Version: notice.Current.Version,
		State:   notice.Current.State.Desired,
	})
}

func previousDesiredDiffers(prev, curr map[string]any) bool {
	return !reflect.DeepEqual(prev, curr)
}

func (h *SimpleHandler) Invoke(ctx context.Context, thingId string, method string, params any, timeout time.Duration) (SimpleInvokeResult, error) {
	online, err := h.connector.IsConnected(thingId)
	if err != nil {
		return SimpleInvokeResult{}, errors.Wrap(err, "check connection")
	}
	if !online {
		return SimpleInvokeResult{}, model.ErrDirectMethodThingOffline
	}

	callID, err := uuid.NewV4()
	if err != nil {
		return SimpleInvokeResult{}, errors.Wrap(err, "generate call ID")
	}
	callIDStr := callID.String()

	req := MethodReq{
		ID:     callIDStr,
		Method: method,
		Data:   params,
	}

	replyCh := make(chan MethodResp, 1)
	h.addPendingCall(thingId, callIDStr, replyCh)
	defer h.removePendingCall(thingId, callIDStr)

	if err := h.publishReq(TopicDown(thingId, TypeMethodReq), req); err != nil {
		return SimpleInvokeResult{}, errors.Wrap(err, "publish method request")
	}

	select {
	case <-time.After(timeout):
		return SimpleInvokeResult{}, model.ErrDirectMethodTimeout
	case <-ctx.Done():
		return SimpleInvokeResult{}, ctx.Err()
	case resp := <-replyCh:
		return SimpleInvokeResult{
			Code:    resp.Code,
			Data:    resp.Data,
			Message: resp.Message,
		}, nil
	}
}

func (h *SimpleHandler) publish(topic string, msg any) {
	payload, err := h.codec.Marshal(msg)
	if err != nil {
		slog.Error("marshal message", "error", err, "topic", topic)
		return
	}
	if err := h.connector.PublishReliable(topic, payload); err != nil {
		slog.Error("publish message", "error", err, "topic", topic)
	}
}

func (h *SimpleHandler) publishReq(topic string, msg any) error {
	payload, err := h.codec.Marshal(msg)
	if err != nil {
		return errors.Wrap(err, "marshal message")
	}
	if err := h.connector.PublishReliable(topic, payload); err != nil {
		return errors.Wrap(err, "publish message")
	}
	return nil
}

func (h *SimpleHandler) addPendingCall(thingId, callID string, ch chan MethodResp) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()

	if h.pendingCalls[thingId] == nil {
		h.pendingCalls[thingId] = make(map[string]chan MethodResp)
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
