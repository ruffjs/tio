package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"ruff.io/tio/auth"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/thing"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

type CreateReq struct {
	ThingId   string `json:"thingId"`
	Password  string `json:"password"`
	IsGateway bool   `json:"IsGateway"`

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

var decoder = schema.NewDecoder()

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
		Param(ws.QueryParameter("isGateway", "").DataType("string")).
		Param(ws.QueryParameter("gatewayThingId", "gateway thingId").DataType("string")).
		Param(ws.QueryParameter("withAuthValue", "whether return authValue field").DataType("boolean")).
		Param(ws.QueryParameter("pageIndex", "page index, from 1").DataType("integer").DefaultValue("1")).
		Param(ws.QueryParameter("pageSize", "page size, from 1").DataType("integer").DefaultValue("10")).
		Returns(200, "OK", rest.RespOK(thing.Page{})))

	ws.Route(ws.POST("/").
		To(CreateHandler(ctx, svc)).
		Operation("create-one").
		Doc("create thing").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.QueryParameter("upsert", "whether upsert thing if it already exists").
			DataType("boolean").DefaultValue("false")).
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
		Reads(thing.ThingPatch{}).
		Returns(200, "OK", rest.RespOK("")))

	ws.Route(ws.POST("/{id}/bind").
		To(BindToGateway(ctx, svc)).
		Operation("bind-to-gw").
		Doc("bind things to the thing which is gateway").
		Notes("One thing can only be bound to one gateway").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "gateway thingId")).
		Reads(thing.ThingBindReq{}).
		Returns(200, "OK", rest.RespOK("")))

	ws.Route(ws.POST("/{id}/unbind").
		To(UnbindFromGateway(ctx, svc)).
		Doc("unbind the things from thing which is gateway").
		Notes("Unbind things in array `thingIds`, if `thingIds` is empty array, unbind all things from the gateway").
		Operation("unbind-from-gw").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("id", "gateway thingId")).
		Reads(thing.ThingBindReq{}).
		Returns(200, "OK", rest.RespOK("")))

	return ws
}

func ServiceForEmqxIntegration(aclFn auth.AclFn) *restful.WebService {
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
			res := aclFn("", thingId, topic, action == "subscribe")
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
		upsert := r.QueryParameter("upsert") == "true"

		th := thing.Thing{
			Id:        cReq.ThingId,
			Enabled:   true,
			AuthType:  thing.AuthTypePassword,
			AuthValue: cReq.Password,
			IsGateway: cReq.IsGateway,
		}
		rTh, err := svc.Create(ctx, th, upsert)
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
		var req thing.ThingPatch
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
			rTh, err := svc.Create(ctx, th, false)
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
		var err error
		q, er := getPgQry(r)
		err = er
		if err == nil {
			if rPg, er := svc.Query(ctx, q); er == nil {
				_ = w.WriteEntity(rest.RespOK(rPg))
				return
			} else {
				err = er
			}
		}
		if !checkHttpErrAndSend(err, w) {
			rest.SendResp(w, 500, rest.Resp[string]{Code: 500, Message: err.Error()})
		}
	}
}

func getPgQry(r *restful.Request) (thing.PageQuery, error) {
	q := thing.PageQuery{}

	r.Request.ParseForm()
	if err := decoder.Decode(&q, r.Request.Form); err != nil {
		return q, errors.WithMessage(model.ErrInvalidParams, err.Error())
	}
	return q, nil
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

func BindToGateway(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		id := r.PathParameter("id")
		var req thing.ThingBindReq
		if err := r.ReadEntity(&req); err != nil {
			slog.Error("Failed to decode body for bind to gateway", "thingId", id, "error", err)
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if err := svc.BindToGateway(ctx, req.ThingIds, id); err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				_ = w.WriteHeaderAndEntity(500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {
			rest.SendRespOK(w, "")
		}
	}
}

func UnbindFromGateway(ctx context.Context, svc thing.Service) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		id := r.PathParameter("id")
		var req thing.ThingBindReq
		if err := r.ReadEntity(&req); err != nil {
			slog.Error("Failed to decode body for unbind from gateway", "thingId", id, "error", err)
			_ = w.WriteHeaderAndEntity(400, rest.Resp[string]{Code: 400, Message: err.Error()})
			return
		}
		if err := svc.UnbindFromGateway(ctx, req.ThingIds, id); err != nil {
			sent := checkHttpErrAndSend(err, w)
			if !sent {
				_ = w.WriteHeaderAndEntity(500, rest.Resp[string]{Code: 500, Message: err.Error()})
			}
		} else {
			rest.SendRespOK(w, "")
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
