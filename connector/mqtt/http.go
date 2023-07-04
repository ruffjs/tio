package mqtt

import (
	"context"

	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/pkg/log"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/shadow"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/mochi-co/mqtt/v2/system"
)

func Service(ctx context.Context, brk shadow.Connectivity) *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/api/v1/mqttBroker").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"mqttBroker"}

	ws.Route(ws.DELETE("/clients/{clientId}").
		To(CloseClientHandler(ctx, brk)).
		Doc("Kick off the client from the mqtt broker").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("clientId", "")).
		Returns(200, "OK", rest.RespOK(rest.H{"deleted": true})))

	ws.Route(ws.GET("/embed/stats").
		To(GetEmbedBrokerStats).
		Doc("Get embedded mqtt broker stats info").
		Notes("The embedded mqtt broker of tio is used `https://github.com/mochi-co/mqtt`. This api is for getting it's stats info").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(system.Info{})))

	return ws
}

func CloseClientHandler(ctx context.Context, connector shadow.Connectivity) restful.RouteFunction {
	return func(r *restful.Request, w *restful.Response) {
		cid := r.PathParameter("clientId")
		err := connector.Close(cid)
		ok := true
		errMsg := "OK"
		if err != nil {
			log.Warnf("Close mqtt client error: %v", err)
			ok = false
			errMsg = err.Error()
		}
		rest.SendResp(w, 200, rest.RespOK(rest.H{"deleted": ok, "message": errMsg}))
	}
}

func GetEmbedBrokerStats(req *restful.Request, resp *restful.Response) {
	if embed.BrokerInstance() == nil {
		rest.SendResp(resp, 200, rest.Resp[any]{Code: 400, Message: "broker is not running"})
		return
	}
	info := *embed.BrokerInstance().StatsInfo().Clone()
	rest.SendResp(resp, 200, rest.RespOK(info))
}
