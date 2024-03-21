package config

import (
	"context"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	rest "ruff.io/tio/pkg/restapi"
)

func Service(ctx context.Context, cfg Config) *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/private/api/config").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"private"}

	ws.Route(ws.GET("").
		To(GetEmbedBrokerConfig(cfg)).
		Operation("get-config").
		Doc("Get tio config").
		Notes("WARN: This api is not for integration").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(Config{})))

	return ws
}

func GetEmbedBrokerConfig(cfg Config) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		rest.SendResp(w, 200, rest.RespOK(cfg))
	}
}
