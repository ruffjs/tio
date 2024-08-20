package api

import (
	"context"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/rule"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/source"
	"ruff.io/tio/shadow"
)

type TestRuleReq struct {
	ThingId        string           `json:"thingId"`
	Topic          string           `json:"topic"`
	Payload        string           `json:"payload"`
	ProcessConfigs []process.Config `json:"processConfigs"`
}

type TestRuleResp struct {
	Success bool   `json:"success"`
	Output  any    `json:"output"`
	Message string `json:"message"`
}

func Service(
	ctx context.Context,
) *restful.WebService {
	ws := new(restful.WebService).
		Path("/api/v1/rules").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"rules"}

	ws.Route(ws.POST("/test").
		To(TestHandler(ctx)).
		Operation("rule-test").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Reads(TestRuleReq{}).
		Returns(200, "OK", rest.RespOK(TestRuleResp{})))

	return ws
}

func TestHandler(ctx context.Context) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var req TestRuleReq
		if err := r.ReadEntity(&req); err != nil {
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}

		processors := make([]process.Process, 0, len(req.ProcessConfigs))
		for _, pc := range req.ProcessConfigs {
			if p, err := process.NewProcess(pc); err == nil {
				processors = append(processors, p)
			} else {
				rest.SendRespOK(w, TestRuleResp{Success: false, Message: err.Error()})
				return
			}
		}

		in, err := rule.MsgToProcessInput(
			source.Msg{ThingId: req.ThingId, Topic: req.Topic, Payload: req.Payload},
			shadow.ShadowWithStatus{Shadow: shadow.Shadow{ThingId: req.ThingId}},
		)
		if err != nil {
			rest.SendRespOK(w, TestRuleResp{Success: false, Message: "message invalid:" + err.Error()})
			return
		}

		if len(processors) == 0 {
			rest.SendRespOK(w, TestRuleResp{Success: true, Output: req.Payload})
			return
		}

		for _, p := range processors {
			if p.Type() == process.TypeFilter {
				pass, err := p.Run(in)
				if err != nil {
					rest.SendRespOK(w, TestRuleResp{Success: false, Message: "run process " + p.Name() + ": " + err.Error()})
					return
				}
				if pass != true {
					rest.SendRespOK(w, TestRuleResp{Success: true, Output: nil, Message: "filtered"})
					return
				}
				continue
			}

			// transform
			if in, err = p.Run(in); err != nil {
				rest.SendRespOK(w, TestRuleResp{Success: false, Message: "run process " + p.Name() + ": " + err.Error()})
				return
			}
		}
		rest.SendRespOK(w, TestRuleResp{Success: true, Output: in})
	}
}
