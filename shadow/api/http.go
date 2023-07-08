package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"ruff.io/tio/pkg/model"
	"ruff.io/tio/shadow"
	"ruff.io/tio/thing"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"ruff.io/tio/pkg/log"
	rest "ruff.io/tio/pkg/restapi"
)

const (
	defaultMethodReqTimeout = 30
)

type ShadowQuery struct {
	Query string `json:"query" description:"SQL-like query string" default:"select * from shadow"`
}

func Service(
	ctx context.Context,
	thingWs *restful.WebService,
	svc shadow.Service,
	thingSvc thing.Service,
	conn shadow.Connector,
) *restful.WebService {
	ws := thingWs

	tags := []string{"shadows"}

	ws.Route(ws.POST("/shadows/query").
		To(QueryHandler(ctx, svc)).
		Operation("query").
		Notes(
			"SQL query string like : select * from shadow where \\`tags.zone\\` = 'Shanghai'.\n"+
				"\nJSON path (eg: tags.Shanghai) must be surrounded with `` .\n"+
				"\nThese fields are queryable: \n"+
				"  - `thingId, createdAt, updatedAt, version`\n"+
				"  - filed about connection: `connected, connectedAt, disconnectedAt, remoteAddr` \n"+
				"  - field under `tags, state.reported, state.desired` , eg: tags.zone, state.reported.loc.lat, sate.desired.x.y\n"+
				"\nThese fields can be used as sorting fields:\n"+
				"  - `thingId, createdAt, updatedAt`\n",
		).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.QueryParameter("pageIndex", "").DefaultValue("1")).
		Param(ws.QueryParameter("pageSize", "").DefaultValue("10")).
		Reads(ShadowQuery{}).
		Returns(200, "OK", rest.RespOK(shadow.Page{})))

	ws.Route(ws.PUT("/{id}/shadows/default/state/desired").
		To(PatchDesiredStateHandler(ctx, svc)).
		Operation("set-state-desired").
		Doc("set shadow desired state").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Reads(shadow.StateReq{}).
		Returns(200, "OK", rest.RespOK("")))

	ws.Route(ws.GET("/{id}/shadows/default").
		To(GetDesiredStateHandler(ctx, svc)).
		Operation("get-one").
		Doc("get shadow").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Returns(200, "OK", rest.RespOK(shadow.ShadowWithStatus{})))

	ws.Route(ws.POST("/{id}/methods/{name}").
		To(InvokeMethodHandler(ctx, conn, thingSvc)).
		Operation("invoke-direct-method").
		Doc("invoke thing direct method").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Param(ws.PathParameter("name", "method name")).
		Reads(MethodInvokeReq{}).
		Returns(200, "OK", rest.RespOK(MethodInvokeResp{Data: struct{}{}})))

	ws.Route(ws.PUT("/{id}/shadows/tags").
		To(SetTagsHandler(ctx, svc)).
		Operation("set-tags").
		Doc("set shadow tags property").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Reads(shadow.TagsReq{}).
		Returns(200, "OK", rest.RespOK("")))

	return ws
}

func GetDesiredStateHandler(ctx context.Context, svc shadow.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		thingId := r.PathParameter("id")
		s, err := svc.Get(ctx, thingId, shadow.GetOption{WithStatus: true})
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				rest.SendResp(w, 404, rest.Resp[any]{Code: 404, Message: err.Error()})
			} else {
				log.Errorf("Error getting shadow: %v ", err)
				rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error()})
			}
		} else {
			rest.SendResp(w, 200, rest.RespOK(s))
		}
	}
}

func QueryHandler(ctx context.Context, svc shadow.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		pq := getPageQuery(r)

		var shadowQuery ShadowQuery
		err := r.ReadEntity(&shadowQuery)
		if err != nil {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		}
		res, err := svc.Query(ctx, pq, shadowQuery.Query)

		if err != nil {
			if !checkHttpErrAndSend(err, w) {
				rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error()})
			}
		} else {
			rest.SendResp(w, 200, rest.RespOK(res))
		}
	}
}

func getPageQuery(r *restful.Request) model.PageQuery {
	var err error
	q := model.PageQuery{}
	q.PageIndex, err = strconv.Atoi(r.QueryParameter("pageIndex"))
	if err != nil {
		q.PageIndex = 1
		log.Infof("No valid query param withAuthValue use default value %d", q.PageIndex)
	}
	q.PageSize, err = strconv.Atoi(r.QueryParameter("pageSize"))
	if err != nil {
		q.PageSize = 10
		log.Infof("No valid query param withAuthValue use default value %d", q.PageSize)
	}

	return q
}

func PatchDesiredStateHandler(ctx context.Context, svc shadow.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		thingId := r.PathParameter("id")
		var stateReq shadow.StateReq
		err := r.ReadEntity(&stateReq)
		if err != nil {
			log.Infof("Bad request to set desired: %v", err)
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		}
		if stateReq.ClientToken == "" || stateReq.State.Desired == nil || stateReq.State.Reported != nil {
			log.Infof("Bad request to set desired: %v, body: %#v", err, stateReq)
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "Invalid request body"})
			return
		}
		_, err = svc.SetDesired(ctx, thingId, stateReq)
		if err != nil {
			log.Errorf("Error setting desired: %v, body: %#v", err, stateReq)
			rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error()})
			return
		}
		rest.SendResp(w, 200, rest.RespOK(""))
	}
}

func SetTagsHandler(ctx context.Context, svc shadow.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		thingId := r.PathParameter("id")
		var tagsReq shadow.TagsReq
		err := r.ReadEntity(&tagsReq)
		if err != nil {
			log.Infof("Bad request to set tags: %v", err)
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		}
		if tagsReq.Tags == nil {
			log.Infof("Bad request to set tags: %v, body: %#v", err, tagsReq)
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "Invalid request body"})
			return
		}
		_, err = svc.SetTag(ctx, thingId, tagsReq)
		if err != nil {
			log.Errorf("Error setting tags: %v, body: %#v", err, tagsReq)
			rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error()})
			return
		}
		rest.SendResp(w, 200, rest.RespOK(""))
	}
}

type MethodInvokeReq struct {
	ConnTimeout int `json:"connTimeout" description:"waiting time for the thing to come online, in seconds"`
	RespTimeout int `json:"respTimeout" description:"waiting time for the thing to response, in seconds"`
	Data        any `json:"data" description:"Any legal json data, including basic types, array, object, etc."`
}

type MethodInvokeResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func InvokeMethodHandler(
	ctx context.Context,
	conn shadow.Connector,
	thingSvc thing.Service,
) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		thingId := r.PathParameter("id")
		name := r.PathParameter("name")
		var req MethodInvokeReq
		err := r.ReadEntity(&req)
		if err != nil {
			log.Infof("Bad request for invoking thing %s method %s : %v", thingId, name, err)
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: err.Error()})
			return
		}
		if req.ConnTimeout > 300 {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "connTimeout should between 0 and 300 second"})
			return
		}
		if req.RespTimeout > 300 {
			rest.SendResp(w, 400, rest.Resp[any]{Code: 400, Message: "respTimeout should between 0 and 300 second"})
			return
		}

		reqMsg := shadow.MethodReqMsg{
			ThingId:     thingId,
			Method:      name,
			ConnTimeout: req.ConnTimeout,
			RespTimeout: req.RespTimeout,
			Req: shadow.MethodReq{
				ClientToken: fmt.Sprintf("tk-sys-%d", time.Now().UnixNano()),
				Data:        req.Data,
			},
		}

		exist, err := thingSvc.Exist(ctx, thingId)
		if err != nil {
			rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error(), Data: nil})
			log.Errorf("Direct method request error: %v , request: %#v", err, reqMsg)
			return
		}
		if !exist {
			rest.SendResp(w, 404, rest.Resp[any]{Code: 404, Message: "thing not found"})
			log.Warnf("Direct method request error: thing not exist , request: %#v", reqMsg)
			return
		}

		if reqMsg.RespTimeout == 0 {
			reqMsg.RespTimeout = defaultMethodReqTimeout
		}

		resp, err := conn.InvokeMethod(ctx, reqMsg)
		if err != nil {
			log.Errorf("Direct method request: %#v , error: %v", reqMsg, err)
			if !checkHttpErrAndSend(err, w) {
				rest.SendResp(w, 500, rest.Resp[any]{Code: 500, Message: err.Error(), Data: nil})
			}
		} else {
			res := MethodInvokeResp{
				Code:    resp.Code,
				Message: resp.Message,
				Data:    resp.Data,
			}
			rest.SendResp(w, 200, rest.RespOK(res))
			log.Debugf("Direct method request: %#v response: %#v", reqMsg, resp)
		}
	}
}

func checkHttpErrAndSend(err error, w http.ResponseWriter) bool {
	if err != nil {
		var he model.HttpErr
		if ok := errors.As(err, &he); ok {
			rest.SendResp(w, he.HttpCode, rest.Resp[any]{Code: he.Code, Message: err.Error()})
			return true
		}
	}
	return false
}
