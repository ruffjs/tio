package mqtt

import (
	"context"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/wind-c/comqtt/v2/mqtt/system"

	"ruff.io/tio/connector/mqtt/embed"
	rest "ruff.io/tio/pkg/restapi"
)

// Service exposes the embedded comqtt broker's stats and clients for debugging.
// The client-kick endpoint lives in api.MQTTBrokerService; these routes only
// report the embedded broker's runtime state.
func Service(ctx context.Context, brk *embed.Broker) *restful.WebService {
	ws := new(restful.WebService)
	ws.
		Path("/api/v1/mqttBroker/embed").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	tags := []string{"mqttBroker"}

	ws.Route(ws.GET("/stats").
		To(getEmbedBrokerStats(brk)).
		Operation("embed-stats").
		Doc("Get embedded mqtt broker stats info").
		Notes("Stats for tio's embedded comqtt MQTT broker.").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(system.Info{})))

	ws.Route(ws.GET("/clients").
		To(getEmbedBrokerClients(brk)).
		Operation("embed-mqtt-clients").
		Doc("Get embedded mqtt broker clients").
		Notes("WARN: This api is not for integration cause it is not ready, it is just for temporary debugging").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Returns(200, "OK", rest.RespOK(embed.Client{})))

	return ws
}

func getEmbedBrokerStats(brk *embed.Broker) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if brk == nil || brk.StatsInfo() == nil {
			rest.SendResp(resp, 200, rest.Resp[any]{Code: 400, Message: "broker is not running"})
			return
		}
		rest.SendResp(resp, 200, rest.RespOK(*brk.StatsInfo().Clone()))
	}
}

func getEmbedBrokerClients(brk *embed.Broker) restful.RouteFunction {
	return func(req *restful.Request, resp *restful.Response) {
		if brk == nil {
			rest.SendResp(resp, 200, rest.Resp[any]{Code: 400, Message: "broker is not running"})
			return
		}
		rest.SendResp(resp, 200, rest.RespOK(brk.AllClients()))
	}
}
