package api

import (
	"context"
	"errors"
	"fmt"
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

type InvalidCreate struct {
	ThingId   string `json:"thingId"`
	ErrorCode string `json:"errorCode"`
	ErrorMsg  string `json:"errorMsg"`
}

type CreateBatchResp struct {
	InvalidList []InvalidCreate `json:"invalidList"`
	ValidList   []thing.Thing   `json:"validList"`
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

func (req CreateReq) batchValidate() error {
	if len(req.ThingId) == 0 {
		return errors.New("batch create thingId can't be empty")
	}

	return req.validate()
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
		Param(ws.QueryParameter("enabled", "whether thing is enabled").DataType("boolean")).
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

	ws.Route(ws.POST("/batch").
		To(CreateBatchHandler(ctx, svc)).
		Operation("create-batch").
		Doc("create things").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Reads([]CreateReq{}).
		Returns(200, "OK", rest.RespOK(CreateBatchResp{})))

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

	ws.Route(ws.PATCH("/{id}").
		To(UpdateHandler(ctx, svc)).
		Operation("update-one").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "thing id")).
		Reads(thing.ThingUpdate{}).
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
			log.Infof("Invalid request for create thing: %v", err)
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

func UpdateHandler(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		id := r.PathParameter("id")
		var req thing.ThingUpdate
		err := r.ReadEntity(&req)
		if err != nil {
			log.Infof("Error decoding body for update thing: %v", err)
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if err := svc.Update(ctx, id, req); err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				_ = w.WriteHeaderAndEntity(500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {
			rest.SendRespOK(w, "")
		}
	}
}

func CreateBatchHandler(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		var resp CreateBatchResp
		var cReq []CreateReq
		err := r.ReadEntity(&cReq)
		if err != nil {
			log.Infof("Error decoding body for create thing: %v", err)
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}

		if len(cReq) > 1000 {
			msg := "In a single call, you can create a maximum of 1000 things"
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: msg})
			return
		}

		for _, req := range cReq {
			err = req.batchValidate()
			if err != nil {
				msg := fmt.Sprintf("Invalid request for create thing: %v", err)
				log.Info(msg)
				resp.InvalidList = append(resp.InvalidList, InvalidCreate{req.ThingId, "Illegal", msg})
				continue
			}

			th := thing.Thing{
				Id:        req.ThingId,
				Enabled:   true,
				AuthType:  thing.AuthTypePassword,
				AuthValue: req.Password,
			}
			rTh, err := svc.Create(ctx, th)
			if err != nil {
				resp.InvalidList = append(resp.InvalidList, InvalidCreate{req.ThingId, "InternalFailureException", err.Error()})
				continue
			}
			resp.ValidList = append(resp.ValidList, rTh)
		}

		_ = w.WriteEntity(rest.RespOK(resp))
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
	if e, err := strconv.ParseBool(r.QueryParameter("enabled")); err == nil {
		q.Enabled = &e
	}
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
