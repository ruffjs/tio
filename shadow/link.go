package shadow

// Link shadow connector to shadow service
// things <--> connector <--> service

import (
	"context"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
)

const (
	maxShadowUpdateWorkerCount = 500
	maxShadowGetWorkerCount    = 500
)

func Link(ctx context.Context, conn StateHandler, svc Service) error {
	// handle shadow get request
	go func() {
		ch, err := conn.ShadowGetReq(ctx)
		if err != nil {
			log.Fatalf("Init shadow get subscribe error %v", err)
		}
		pool, err := ants.NewPoolWithFunc(maxShadowGetWorkerCount, func(shadowGetReq any) {
			req := shadowGetReq.(GetReqMsg)
			handleShadowGetReq(ctx, svc, conn, req)
		})
		if err != nil {
			log.Fatalf("New pool for shadow get : %v", err)
		}
		log.Info("Link shadow get request initialized")
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-ch:
				if !ok {
					log.Errorf("Shadow get request channel closed")
					return
				}
				pool.Invoke(req)
			}
		}
	}()

	// handle shadow update request
	go func() {
		ch, err := conn.StateUpdateReq(ctx)
		if err != nil {
			log.Fatalf("Init shadow state update subscribe error %v", err)
		}
		pool, err := ants.NewPoolWithFunc(maxShadowUpdateWorkerCount, func(stateReqMsg any) {
			req := stateReqMsg.(StateReqMsg)
			handleShadowStateUpdateReq(ctx, svc, conn, req)
		})
		if err != nil {
			log.Debugf("New pool for shadow update: %v", err)
		}
		log.Info("Link shadow update request initialized")
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-ch:
				if !ok {
					log.Errorf("Shadow state update channel closed")
					return
				}
				pool.Invoke(req)
			}
		}
	}()

	svc.SubscribeDelta(func(thingId string, delta DeltaStateNotice) {
		msg := DeltaStateNoticeMsg{ThingId: thingId, Notice: delta}
		err := conn.StateDeltaNotify(ctx, msg)
		if err != nil {
			log.Errorf("Notify state delta error: %v, msg: %#v", err, msg)
		} else {
			log.Debugf("Notify state delta msg: %#v", msg)
		}
	})

	svc.SubscribeUpdate(func(thingId string, notice StateUpdatedNotice) {
		msg := StateUpdatedNoticeMsg{ThingId: thingId, Notice: notice}
		err := conn.StateUpdatedNotify(ctx, msg)
		if err != nil {
			log.Errorf("Notify state update error: %v, msg: %#v", err, msg)
		} else {
			log.Debugf("Notify state update msg: %#v", msg)
		}
	})

	svc.SubAccepted(func(thingId string, msg StateAcceptedRespMsg) {
		err := conn.AcceptedResp(ctx, msg)
		if err != nil {
			log.Errorf("Notify state accepted error: %v, msg: %#v", err, msg)
		} else {
			log.Debugf("Notify state accepted msg: %#v", msg)
		}
	})
	svc.SubRejected(func(thingId string, msg ErrRespMsg) {
		err := conn.RejectedResp(ctx, msg)
		if err != nil {
			log.Errorf("Notify state rejected error: %v, msg: %#v", err, msg)
		} else {
			log.Debugf("Notify state rejected msg: %#v", msg)
		}
	})
	return nil
}

func handleShadowStateUpdateReq(ctx context.Context, svc Service, h StateHandler, req StateReqMsg) {
	_, err := svc.SetReported(ctx, req.ThingId, req.Req)
	// just handle rejected msg
	if err == nil {
		return
	}
}

func handleShadowGetReq(ctx context.Context, svc Service, h StateHandler, req GetReqMsg) {
	ss, err := svc.Get(ctx, req.ThingId, GetOption{})
	if err != nil {
		resp := ErrResp{ClientToken: req.Req.ClientToken, Timestamp: time.Now().UnixMilli()}
		resp.Message = err.Error()
		if errors.Is(err, model.ErrNotFound) {
			resp.Code = 404
		} else if errors.Is(err, model.ErrShadowFormat) {
			resp.Code = 400
		}
		msg := ErrRespMsg{ThingId: req.ThingId, Op: OpGet, Resp: resp}
		e := h.RejectedResp(ctx, msg)
		if e != nil {
			log.Errorf("Send rejected msg error %v, msg: %#v", e, msg)
		}
		return
	}
	delta, _ := DeltaState(ss.State.Desired, ss.State.Reported, nil)
	resp := StateAcceptedResp{
		Timestamp:   time.Now().UnixMilli(),
		ClientToken: req.Req.ClientToken,
		State: StateDRD{
			Desired:  ss.State.Desired,
			Reported: ss.State.Reported,
			Delta:    delta,
		},
		Metadata: ss.Metadata,
		Version:  ss.Version,
	}
	msg := StateAcceptedRespMsg{ThingId: req.ThingId, Op: OpGet, Resp: resp}
	err = h.AcceptedResp(ctx, msg)
	if err != nil {
		log.Errorf("Send accepted msg error %v, msg: %#v", err, msg)
	}
}
