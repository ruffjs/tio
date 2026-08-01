package shadow

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/pkg/model"
)

const (
	TopicMethodPrefix = TopicThingsPrefix + "{thingId}/methods/{methodName}"
	TopicMethodReq    = "/req"
	TopicMethodResp   = "/resp"
)

type MethodReqMsg struct {
	ThingId     string `json:"thingId"`
	Method      string `json:"method"`
	ConnTimeout int    `json:"connTimeout"`     // seconds
	RespTimeout int    `json:"responseTimeout"` // seconds
	Req         MethodReq
}

type MethodReq struct {
	ClientToken string `json:"clientToken,omitempty"`
	Data        any    `json:"data,omitempty"`
}

type MethodResp struct {
	ClientToken string `json:"clientToken,omitempty"`
	Data        any    `json:"data,omitempty"`
	Code        int    `json:"code"`
	Message     string `json:"message"`
}

type MethodRespMsg struct {
	ThingId string `json:"thingId"`
	Resp    MethodResp
}

type MethodHandler interface {
	InvokeMethod(ctx context.Context, req MethodReqMsg) (MethodResp, error)
	InitMethodHandler(ctx context.Context) error
}

func TopicMethodRequest(thingId, methodName string) string {
	return topicMethodPrefix(thingId, methodName) + TopicMethodReq
}

func TopicMethodResponse(thingId, methodName string) string {
	return topicMethodPrefix(thingId, methodName) + TopicMethodResp
}

func TopicMethodAllResponse() string {
	s := strings.Replace(TopicMethodPrefix, "{thingId}", "+", -1) + TopicMethodResp
	s = strings.Replace(s, "{methodName}", "+", -1)
	return s
}

func topicMethodPrefix(thingId, methodName string) string {
	s := strings.Replace(TopicMethodPrefix, "{thingId}", thingId, -1)
	s = strings.Replace(s, "{methodName}", methodName, -1)
	return s
}

// implement

type mqttMethod struct {
	connector connector.Connector
	codec     codec.Codec
	pending   sync.Map
}

type pendingResp struct {
	respChan chan MethodResp
	done     chan struct{}
}

var _ MethodHandler = (*mqttMethod)(nil)

func NewMethodHandler(conn connector.Connector, c codec.Codec) MethodHandler {
	return &mqttMethod{connector: conn, codec: c}
}

func (h *mqttMethod) InitMethodHandler(ctx context.Context) error {
	err := h.subscribeMethodResp(ctx)
	if err != nil {
		return errors.Wrap(err, "start method handler")
	} else {
		slog.Info("Method response subscribe started")
	}
	return nil
}

func (h *mqttMethod) InvokeMethod(
	ctx context.Context,
	msg MethodReqMsg,
) (MethodResp, error) {
	online, err := h.connector.IsConnected(msg.ThingId)
	if err != nil {
		return MethodResp{}, errors.WithMessage(err, "could not get online status")
	}
	if !online {
		return MethodResp{}, model.ErrDirectMethodThingOffline
	}
	return h.doInvokeMethod(ctx, msg)
}

func (h *mqttMethod) doInvokeMethod(ctx context.Context,
	msg MethodReqMsg,
) (MethodResp, error) {
	topic := TopicMethodRequest(msg.ThingId, msg.Method)
	j, err := h.codec.Marshal(msg.Req)
	if err != nil {
		return MethodResp{}, errors.WithMessage(err, "request json marshal")
	}

	outCh := h.addPending(msg.ThingId, msg.Req.ClientToken)
	defer h.removePending(msg.ThingId, msg.Req.ClientToken)

	if err = h.connector.PublishReliable(topic, j); err != nil {
		return MethodResp{}, errors.WithMessage(err, "send method request")
	}
	//ok := token.WaitTimeout(time.Second * time.Duration(msg.RespTimeout))
	//if !ok {
	//	return MethodResp{}, errors.Errorf("send request timeout in %d seconds", msg.RespTimeout)
	//}

	select {
	case <-time.After(time.Second * time.Duration(msg.RespTimeout)):
		return MethodResp{},
			errors.Wrapf(model.ErrDirectMethodTimeout, "wait %d seconds for response", msg.RespTimeout)
	case <-ctx.Done():
		return MethodResp{}, errors.Errorf("interrupted by context done")
	case res, ok := <-outCh:
		if !ok {
			return MethodResp{}, errors.Errorf("out channel closed")
		}
		return res, nil
	}
}

func (h *mqttMethod) removePending(thingId, clientToken string) {
	if pResp, ok := h.pending.Load(thingId); ok {
		if tkResp, ok := pResp.(*sync.Map).Load(clientToken); ok {
			close(tkResp.(pendingResp).done)
			pResp.(*sync.Map).Delete(clientToken)
		}
	}
}

func (h *mqttMethod) addPending(thingId, clientToken string) <-chan MethodResp {
	outCh := make(chan MethodResp)
	tokenMap, _ := h.pending.LoadOrStore(thingId, new(sync.Map))
	tokenMap.(*sync.Map).Store(clientToken, pendingResp{respChan: outCh, done: make(chan struct{})})
	return outCh
}

func (h *mqttMethod) subscribeMethodResp(ctx context.Context) error {
	topic := TopicMethodAllResponse()
	err := h.connector.Subscribe(ctx, topic, func(msg connector.Message) {
		go func() {
			thingId, err := model.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				slog.Error("Got wrong topic msg topic for method response")
				return
			}
			var r MethodResp
			err = h.codec.Unmarshal(msg.Payload(), &r)
			if err != nil {
				slog.Error("Invalid message payload for method response")
				return
			}
			res := MethodRespMsg{
				ThingId: thingId,
				Resp:    r,
			}
			h.sendResp(ctx, res)
		}()
	})

	return err
}

func (h *mqttMethod) sendResp(ctx context.Context, msg MethodRespMsg) {
	if tokenMap, ok := h.pending.Load(msg.ThingId); ok {
		resp, ok := tokenMap.(*sync.Map).Load(msg.Resp.ClientToken)
		if !ok {
			slog.Warn("Method response got no request", "thingId", msg.ThingId, "clientToken", msg.Resp.ClientToken)
			return
		}
		pResp := resp.(pendingResp)
		select {
		case <-pResp.done:
		case pResp.respChan <- msg.Resp:
		case <-ctx.Done():
		}
	} else {
		slog.Warn("Method response got no request", "thingId", msg.ThingId, "clientToken", msg.Resp.ClientToken)
	}
}
