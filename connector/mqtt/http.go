package mqtt

import (
	"context"

	"ruff.io/tio/connector"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/mochi-mqtt/server/v2/system"
	"ruff.io/tio/connector/mqtt/embed"
	"ruff.io/tio/pkg/log"
	rest "ruff.io/tio/pkg/restapi"
)

func Service(ctx context.Context, brk connector.Connectivity) *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/api/v1/mqttBroker").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"mqttBroker"}

	ws.Route(ws.DELETE("/clients/{clientId}").
		To(CloseClientHandler(ctx, brk)).
		Operation("delete-client").
		Doc("Kick off the client from the mqtt broker").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.PathParameter("clientId", "")).
		Returns(200, "OK", rest.RespOK(rest.H{"deleted": true})))

	ws.Route(ws.GET("/embed/stats").
		To(GetEmbedBrokerStats).
		Operation("embed-stats").
		Doc("Get embedded mqtt broker stats info").
		Notes("The embedded mqtt broker of tio is used `https://github.com/mochi-mqtt/server`. This api is for getting it's stats info").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(system.Info{})))

	ws.Route(ws.GET("/embed/clients").
		To(GetEmbedBrokerClients).
		Operation("embed-mqtt-clients").
		Doc("Get embedded mqtt broker clients").
		Notes("WARN: This api is not for integration cause it is not ready, it is just for temporary debugging").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(embed.Client{})))

	return ws
}

func CloseClientHandler(ctx context.Context, connector connector.Connectivity) restful.RouteFunction {
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

func GetEmbedBrokerClients(req *restful.Request, resp *restful.Response) {
	if embed.BrokerInstance() == nil {
		rest.SendResp(resp, 200, rest.Resp[any]{Code: 400, Message: "broker is not running"})
		return
	}
	l := embed.BrokerInstance().AllClients()
	rest.SendResp(resp, 200, rest.RespOK(l))
}
