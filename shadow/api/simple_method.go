package api

import (
	"context"
	"log/slog"
	"time"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/thing"
)

type SimpleInvoker interface {
	Invoke(ctx context.Context, thingId, method string, params any, timeout time.Duration) (any, error)
}

type SimpleInvokeReq struct {
	Method  string `json:"method" description:"method name"`
	Params  any    `json:"params,omitempty" description:"opaque method parameters"`
	Timeout int    `json:"timeout,omitempty" description:"timeout in seconds, default 30"`
}

func SimpleMethodService(
	ctx context.Context,
	thingWs *restful.WebService,
	invoker SimpleInvoker,
	thingSvc thing.Service,
) *restful.WebService {
	ws := thingWs
	tags := []string{"shadows"}

	ws.Route(ws.POST("/{id}/invoke").
		To(SimpleInvokeHandler(ctx, invoker, thingSvc)).
		Operation("simple-invoke").
		Doc("invoke thing method (simple protocol)").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Reads(SimpleInvokeReq{}).
		Returns(200, "OK", rest.RespOK(map[string]any{})))

	return ws
}

func SimpleInvokeHandler(
	ctx context.Context,
	invoker SimpleInvoker,
	thingSvc thing.Service,
) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		thingId := r.PathParameter("id")
		var req SimpleInvokeReq
		if err := r.ReadEntity(&req); err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		}
		if req.Method == "" {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "method is required"})
			return
		}
		if req.Timeout < 0 || req.Timeout > 300 {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "timeout should be between 0 and 300 seconds"})
			return
		}

		exist, err := thingSvc.Exist(ctx, thingId)
		if err != nil {
			slog.Error("Simple invoke error", "error", err, "thingId", thingId)
			rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error()})
			return
		}
		if !exist {
			rest.SendResp(w, 404, rest.Resp[any]{Code: 404, Message: "thing not found"})
			return
		}

		timeout := time.Duration(req.Timeout) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second
		}

		result, err := invoker.Invoke(r.Request.Context(), thingId, req.Method, req.Params, timeout)
		if err != nil {
			slog.Error("Simple invoke", "thingId", thingId, "method", req.Method, "error", err)
			if !checkHttpErrAndSend(err, w) {
				rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error()})
			}
			return
		}

		rest.SendResp(w, 200, rest.RespOK(result))
	}
}
