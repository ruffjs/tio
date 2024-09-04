package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"ruff.io/tio/pkg/model"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/rule"
	"ruff.io/tio/rule/process"
	"ruff.io/tio/rule/source"
	"ruff.io/tio/shadow"
)

type RuleConfigWithStatus struct {
	Config rule.Config         `json:"config"`
	Status rule.RuleStatusInfo `json:"status,omitempty"`
}

type ToggleRule struct {
	Enable bool `json:"enable"`
}

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
	ruleMgr *rule.RuleMgr,
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
		To(GetConfigHandler(ruleMgr)).
		Operation("get-ruel-config").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(rule.Config{})))

	ws.Route(ws.PUT("/config").
		To(SaveConfigHandler(ruleMgr)).
		Operation("save-ruel-config").
		Reads(rule.Config{}).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(rule.Config{})))

	ws.Route(ws.PUT("/{type}/{name}").
		To(ToggleHandler(ruleMgr)).
		Operation("toggle-ruel").
		Param(restful.PathParameter("type", "connector | source | sink | rule").DataType("string")).
		Param(restful.PathParameter("name", "").DataType("string")).
		Reads(ToggleRule{}).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK[any](nil)))

	return ws
}

func GetConfigHandler(ruleMgr *rule.RuleMgr) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		s := r.QueryParameter("withStatus")
		if s != "" {
			rest.SendRespOK(w, RuleConfigWithStatus{ruleMgr.GetConfig(), ruleMgr.GetStatus()})
			return
		} else {
			rest.SendRespOK(w, RuleConfigWithStatus{Config: ruleMgr.GetConfig()})
		}
	}
}

func SaveConfigHandler(ruleMgr *rule.RuleMgr) restful.RouteFunction {
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

		if err := ruleMgr.ApplyConfig(cfg); err != nil {
			checkErrAndSend(err, w)
		} else {
			rest.SendRespOK(w, ruleMgr.GetStatus())
		}
	}
}

func ToggleHandler(ruleMgr *rule.RuleMgr) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		typ := r.PathParameter("type")
		name := r.PathParameter("name")
		var tg ToggleRule
		if err := r.ReadEntity(&tg); err != nil {
			rest.SendResp(w, 400, rest.Resp[string]{Code: 400, Message: err.Error()})
		}
		err := ruleMgr.Enable(typ, name, tg.Enable)
		if err != nil {
			checkErrAndSend(err, w)
			slog.Error("Rule toogle enable component failed", "type", typ, "name", name, "enable", tg.Enable, "error", err)
		} else {
			rest.SendRespOK[any](w, nil)
			slog.Info("Rule component enabled", "type", typ, "name", name, "enable", tg.Enable)
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
