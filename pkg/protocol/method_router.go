package protocol

import (
	"context"
	"time"

	"github.com/gofrs/uuid"
	"ruff.io/tio/connector"
	"ruff.io/tio/pkg/codec"
	"ruff.io/tio/shadow"
)

type MethodRouter struct {
	Profiles connector.DeviceProfileGetter
	Legacy   shadow.MethodHandler
	Simple   *SimpleHandler
}

func (r *MethodRouter) InitMethodHandler(context.Context) error { return nil }

func (r *MethodRouter) InvokeMethod(ctx context.Context, req shadow.MethodReqMsg) (shadow.MethodResp, error) {
	p, err := r.Profiles.GetDeviceProfile(ctx, req.ThingId)
	if err != nil {
		return shadow.MethodResp{}, err
	}
	c, err := codec.New(p.PayloadEncoding)
	if err != nil {
		return shadow.MethodResp{}, err
	}
	ctx = codec.WithSnapshot(ctx, c)
	if p.MessageProtocol == "legacy" {
		return r.Legacy.InvokeMethod(ctx, req)
	}
	resp, err := r.Simple.Invoke(ctx, req.ThingId, req.Method, req.Req.Data, time.Duration(req.RespTimeout)*time.Second)
	return shadow.MethodResp{Code: resp.Code, Message: resp.Message, Data: resp.Data}, err
}

func (r *MethodRouter) Invoke(ctx context.Context, thingId, method string, params any, timeout time.Duration) (SimpleInvokeResult, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return SimpleInvokeResult{}, err
	}
	resp, err := r.InvokeMethod(ctx, shadow.MethodReqMsg{
		ThingId: thingId, Method: method, RespTimeout: int(timeout / time.Second),
		Req: shadow.MethodReq{ClientToken: id.String(), Data: params},
	})
	return SimpleInvokeResult{Code: resp.Code, Message: resp.Message, Data: resp.Data}, err
}
