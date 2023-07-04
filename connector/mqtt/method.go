package mqtt

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	mqcl "ruff.io/tio/connector/mqtt/client"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
)

type mqttMethod struct {
	client      mqcl.Client
	connChecker shadow.ConnectChecker
	pending     sync.Map // thingId -> clientToken -> pendingResp, pending for response receive
	waiting     sync.Map // thingId -> clientToken -> waitingResp, waiting for thing be online
}

type pendingResp struct {
	respChan chan shadow.MethodResp
	done     chan struct{}
}

type waitingResp struct {
	respChan chan bool
	done     chan struct{}
}

var _ shadow.MethodHandler = (*mqttMethod)(nil)

func NewMethodHandler(client mqcl.Client, statusGetter shadow.ConnectChecker) shadow.MethodHandler {
	m := &mqttMethod{
		client:      client,
		connChecker: statusGetter,
	}
	return m
}

func (h *mqttMethod) InitMethodHandler(ctx context.Context) error {
	err := h.subscribeMethodResp(ctx)
	if err != nil {
		return errors.Wrap(err, "start method handler")
	} else {
		log.Infof("Method response subscribe started")
	}
	h.subscribeThingOnline(ctx)
	log.Info("Method thing online subscribe started")
	return nil
}

func (h *mqttMethod) InvokeMethod(
	ctx context.Context,
	msg shadow.MethodReqMsg,
) (shadow.MethodResp, error) {
	online, err := h.connChecker.IsConnected(msg.ThingId)
	if err != nil {
		return shadow.MethodResp{}, errors.WithMessage(err, "could not get online status")
	}
	if online {
		return h.doInvokeMethod(ctx, msg)
	}
	if msg.ConnTimeout <= 0 {
		return shadow.MethodResp{}, model.ErrDirectMethodThingOffline
	}

	// wait for the thing to be online.
	outCh := h.addWaiting(msg.ThingId, msg.Req.ClientToken)
	defer h.removeWaiting(msg.ThingId, msg.Req.ClientToken)

	select {
	case <-time.After(time.Second * time.Duration(msg.ConnTimeout)):
		return shadow.MethodResp{},
			errors.Wrapf(model.ErrDirectMethodTimeout, "wait %d seconds for thing online", msg.ConnTimeout)
	case <-ctx.Done():
		return shadow.MethodResp{}, errors.Errorf("interrupted by context done")
	case online, ok := <-outCh:
		if !ok {
			return shadow.MethodResp{}, errors.Errorf("out channel closed")
		}
		if online {
			// wait the thing to subscribe method request topic
			time.Sleep(time.Millisecond * 500)
			return h.doInvokeMethod(ctx, msg)
		} else {
			return shadow.MethodResp{}, errors.Errorf("out channel returned by thing is offline")
		}
	}
}

func (h *mqttMethod) doInvokeMethod(ctx context.Context,
	msg shadow.MethodReqMsg,
) (shadow.MethodResp, error) {
	topic := shadow.TopicMethodRequest(msg.ThingId, msg.Method)
	j, err := json.Marshal(msg.Req)
	if err != nil {
		return shadow.MethodResp{}, errors.WithMessage(err, "request json marshal")
	}

	outCh := h.addPending(msg.ThingId, msg.Req.ClientToken)
	defer h.removePending(msg.ThingId, msg.Req.ClientToken)

	token := h.client.Publish(topic, DefaultQos, false, j)
	ok := token.WaitTimeout(time.Second * time.Duration(msg.RespTimeout))
	if !ok {
		return shadow.MethodResp{}, errors.Errorf("send request timeout in %d seconds", msg.RespTimeout)
	}

	select {
	case <-time.After(time.Second * time.Duration(msg.RespTimeout)):
		return shadow.MethodResp{},
			errors.Wrapf(model.ErrDirectMethodTimeout, "wait %d seconds for response", msg.RespTimeout)
	case <-ctx.Done():
		return shadow.MethodResp{}, errors.Errorf("interrupted by context done")
	case res, ok := <-outCh:
		if !ok {
			return shadow.MethodResp{}, errors.Errorf("out channel closed")
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

func (h *mqttMethod) addPending(thingId, clientToken string) <-chan shadow.MethodResp {
	outCh := make(chan shadow.MethodResp)
	tokenMap, _ := h.pending.LoadOrStore(thingId, new(sync.Map))
	tokenMap.(*sync.Map).Store(clientToken, pendingResp{respChan: outCh, done: make(chan struct{})})
	return outCh
}

func (h *mqttMethod) removeWaiting(thingId, clientToken string) {
	if pResp, ok := h.waiting.Load(thingId); ok {
		if tkResp, ok := pResp.(*sync.Map).Load(clientToken); ok {
			close(tkResp.(waitingResp).done)
			pResp.(*sync.Map).Delete(clientToken)
		}
	}
}

// return a chan to receive if thing is connected
func (h *mqttMethod) addWaiting(thingId, clientToken string) <-chan bool {
	outCh := make(chan bool)
	tokenMap, _ := h.waiting.LoadOrStore(thingId, new(sync.Map))
	tokenMap.(*sync.Map).Store(clientToken, waitingResp{respChan: outCh, done: make(chan struct{})})
	return outCh
}

func (h *mqttMethod) subscribeThingOnline(ctx context.Context) {
	presenceEvtCh := h.connChecker.OnConnect()
	go func() {
		for e := range presenceEvtCh {
			if e.EventType == shadow.EventConnected {
				if tokenResp, ok := h.waiting.Load(e.ThingId); ok {
					// notify all request (one request mapping to one client token) that thing is online
					tokenResp.(*sync.Map).Range(func(key, value any) bool {
						v := value.(waitingResp)
						select {
						case <-v.done:
						case v.respChan <- true:
						case <-ctx.Done():
						}
						return true
					})
				}
			}
		}
	}()
}

func (h *mqttMethod) subscribeMethodResp(ctx context.Context) error {
	topic := shadow.TopicMethodAllResponse()
	err := h.client.Subscribe(ctx, topic, DefaultQos, func(client mqtt.Client, msg mqtt.Message) {
		go func() {
			thingId, err := shadow.GetThingIdFromTopic(msg.Topic())
			if err != nil {
				log.Errorf("Got wrong topic msg topic for method response")
				return
			}
			var r shadow.MethodResp
			err = json.Unmarshal(msg.Payload(), &r)
			if err != nil {
				log.Errorf("Invalid message payload for method response")
				return
			}
			res := shadow.MethodRespMsg{
				ThingId: thingId,
				Resp:    r,
			}
			h.sendResp(ctx, res)
		}()
	})

	return err
}

func (h *mqttMethod) sendResp(ctx context.Context, msg shadow.MethodRespMsg) {
	if tokenMap, ok := h.pending.Load(msg.ThingId); ok {
		resp, ok := tokenMap.(*sync.Map).Load(msg.Resp.ClientToken)
		if !ok {
			log.Warnf("Method response got no request, thingId=%v clientToken=%s", msg.ThingId, msg.Resp.ClientToken)
			return
		}
		pResp := resp.(pendingResp)
		select {
		case <-pResp.done:
		case pResp.respChan <- msg.Resp:
		case <-ctx.Done():
		}
	} else {
		log.Warnf("Method response got no request, thingId=%v clientToken=%s", msg.ThingId, msg.Resp.ClientToken)
	}
}
