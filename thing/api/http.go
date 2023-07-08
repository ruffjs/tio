package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/thing"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
)

type CreateReq struct {
	ThingId  string `json:"thingId"`
	Password string `json:"password"`

	// AuthType string `json:"authType"`
}

func (req CreateReq) validate() error {
	if len(req.ThingId) > 64 {
		return errors.New("thingId length must be less than 64")
	}
	if len(req.Password) > 64 {
		return errors.New("password length must be less than 64")
	}
	if strings.TrimSpace(req.ThingId) != req.ThingId ||
		strings.TrimSpace(req.Password) != req.Password {
		return errors.New("thingId and password can't contain space character")
	}
	return nil
}

func Service(ctx context.Context, svc thing.Service) *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/api/v1/things").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"things"}

	ws.Route(ws.GET("/").
		To(QueryHandler(ctx, svc)).
		Operation("query").
		Doc("get all things").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.QueryParameter("withAuthValue", "whether return authValue field").DataType("boolean")).
		Param(ws.QueryParameter("withStatus", "whether return fields of status").DataType("boolean")).
		Param(ws.QueryParameter("pageIndex", "page index, from 1").DataType("integer").DefaultValue("1")).
		Param(ws.QueryParameter("pageSize", "page size, from 1").DataType("integer").DefaultValue("10")).
		Returns(200, "OK", rest.RespOK(thing.Page{})))

	ws.Route(ws.POST("/").
		To(CreateHandler(ctx, svc)).
		Operation("create-one").
		Doc("create thing").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Reads(CreateReq{}).
		Returns(200, "OK", rest.RespOK(thing.Thing{})))

	ws.Route(ws.GET("/{id}").
		To(GetHandler(ctx, svc)).
		Operation("get-one").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Returns(200, "OK", rest.RespOK(thing.Thing{})))

	ws.Route(ws.DELETE("/{id}").
		To(DeleteHandler(ctx, svc)).
		Operation("delete-one").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Returns(200, "OK", rest.RespOK("")))

	return ws
}

func ServiceForEmqxIntegration() *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/private/api/things").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"private"}

	ws.Route(ws.GET("/{id}/topicAcl").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Notes("for emqx integration topic acl").
		Param(ws.PathParameter("id", "thing id")).
		Param(ws.QueryParameter("topic", "").DataType("string").Required(true)).
		Param(ws.QueryParameter("action", "").DataType("string").
			PossibleValues([]string{"publish", "subscribe"}).Required(true)).
		Operation("topic-acl").
		To(func(r *restful.Request, w *restful.Response) {
			thingId := r.PathParameter("id")
			topic := r.QueryParameter("topic")
			action := r.QueryParameter("action")
			if thingId == "" || topic == "" || action == "" {
				_ = w.WriteHeaderAndJson(400, "", "'")
				return
			}
			res := thing.TopicAcl(nil, thingId, topic, action == "subscribe")
			resTxt := "deny"
			if res {
				resTxt = "allow"
			}
			_ = w.WriteAsJson(map[string]string{"result": resTxt})
		}))

	return ws
}

func CreateHandler(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var cReq CreateReq
		err := r.ReadEntity(&cReq)
		if err != nil {
			log.Infof("Error decoding body for create thing: %v", err)
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		err = cReq.validate()
		if err != nil {
			log.Info("Invalid request for create thing: %v", err)
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}

		th := thing.Thing{
			Id:        cReq.ThingId,
			Enabled:   true,
			AuthType:  thing.AuthTypePassword,
			AuthValue: cReq.Password,
		}
		rTh, err := svc.Create(ctx, th)
		if err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				_ = w.WriteHeaderAndEntity(500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {
			_ = w.WriteEntity(rest.RespOK(rTh))
		}
	}
}

func QueryHandler(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		rPg, err := svc.Query(ctx, getPgQry(r))
		if err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				rest.SendResp(w, 500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {

			_ = w.WriteEntity(rest.RespOK(rPg))
			// rest.SendResp(w, 200, rest.RespOK(rPg))
		}
	}
}

func getPgQry(r *restful.Request) thing.PageQuery {
	var err error
	q := thing.PageQuery{}
	q.WithAuthValue, _ = strconv.ParseBool(r.QueryParameter("withAuthValue"))
	q.WithStatus, _ = strconv.ParseBool(r.QueryParameter("withStatus"))
	q.PageQuery.PageIndex, err = strconv.Atoi(r.QueryParameter("pageIndex"))
	if err != nil {
		q.PageIndex = 1
		log.Infof("No valid query param withAuthValue use default value %d", q.PageIndex)
	}
	q.PageQuery.PageSize, err = strconv.Atoi(r.QueryParameter("pageSize"))
	if err != nil {
		q.PageSize = 10
		log.Infof("No valid query param withAuthValue use default value %d", q.PageSize)
	}
	return q
}

func GetHandler(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		id := r.PathParameter("id")
		rPg, err := svc.Get(ctx, id)
		if err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				rest.SendResp(w, 500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {
			rest.SendResp(w, 200, rest.RespOK(rPg))
		}
	}
}

func DeleteHandler(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		id := r.PathParameter("id")
		log.Infof("To delete thing %q", id)
		err := svc.Delete(ctx, id)
		if err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				rest.SendResp(w, 500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {
			log.Infof("Deleted thing %q", id)
			rest.SendResp(w, 200, rest.RespOK(""))
		}
	}
}

func checkHttpErrAndSend(err error, w http.ResponseWriter) bool {
	if err != nil {
		var he model.HttpErr
		if ok := errors.As(err, &he); ok {
			rest.SendResp(w, he.HttpCode, rest.Resp[string]{Code: he.Code, Message: err.Error()})
			return true
		}
	}
	return false
}
