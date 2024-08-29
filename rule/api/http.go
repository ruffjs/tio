package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"ruff.io/tio/config"
	"ruff.io/tio/pkg/model"
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

	ws.Route(ws.GET("/config").
		To(GetConfigHandler()).
		Operation("get-ruel-config").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(rule.Config{})))

	ws.Route(ws.PUT("/config").
		To(SaveConfigHandler()).
		Operation("save-ruel-config").
		Reads(rule.Config{}).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(rule.Config{})))

	return ws
}

func GetConfigHandler() restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		rest.SendRespOK(w, rule.GetConfig())
	}
}

func SaveConfigHandler() restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var cfg rule.Config

		// Bug found by code below: int value in connector options (map[string]any) is be converted to string
		// if err := r.ReadEntity(&cfg); err != nil {
		// 	rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
		// 	return
		// }

		if err := json.NewDecoder(r.Request.Body).Decode(&cfg); err != nil {
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}

		if err := rule.SetConfig(cfg); err != nil {
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK(w, rule.GetConfig())

			// TODO optimize this by hot reload rule
			go func() {
				config.GlobalCtxCancel()
			}()
		}
	}
}

func checkErrAndSend(err error, w http.ResponseWriter) {
	var he model.HttpErr
	if ok := errors.As(err, &he); ok {
		rest.SendResp(w, he.HttpCode, rest.Resp[string]{Code: he.Code, Message: err.Error()})
	} else {
		rest.SendResp(w, 500, rest.Resp[string]{Code: 500, Message: err.Error()})
	}
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
