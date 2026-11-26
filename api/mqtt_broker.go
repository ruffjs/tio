package api

import (
	"log/slog"
	"net/http"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/thing"
)

func MQTTBrokerService(closeAll func(string) error, things thing.Service, monitor BrokerMonitor) *restful.WebService {
	ws := new(restful.WebService).Path("/api/v1/mqttBroker").Produces(restful.MIME_JSON)
	ws.Route(ws.DELETE("/clients/{id}").
		To(func(req *restful.Request, resp *restful.Response) {
			id := req.PathParameter("id")
			if !thing.IdValid(id) {
				rest.SendResp(resp, http.StatusBadRequest, rest.Resp[any]{Code: http.StatusBadRequest, Message: "invalid thing ID"})
				return
			}
			exists, err := things.Exist(req.Request.Context(), id)
			if err != nil {
				slog.Error("Check thing before disconnect", "thingId", id, "error", err)
				rest.SendResp(resp, http.StatusInternalServerError, rest.Resp[any]{Code: http.StatusInternalServerError, Message: "failed to check thing"})
				return
			}
			if !exists {
				rest.SendResp(resp, http.StatusNotFound, rest.Resp[any]{Code: http.StatusNotFound, Message: "thing not found"})
				return
			}
			if err := closeAll(id); err != nil {
				slog.Error("Disconnect MQTT client", "thingId", id, "error", err)
				rest.SendResp(resp, http.StatusInternalServerError, rest.Resp[any]{Code: http.StatusInternalServerError, Message: "failed to disconnect thing"})
				return
			}
			rest.SendRespOK(resp, rest.H{"deleted": true, "message": "OK"})
		}).
		Operation("delete-client").
		Doc("Disconnect an MQTT client by thing ID").
		Metadata(restfulspec.KeyOpenAPITags, []string{"mqttBroker"}).
		Param(ws.PathParameter("id", "Thing ID").DataType("string")).
		Returns(http.StatusOK, "OK", rest.RespOK(rest.H{"deleted": true, "message": "OK"})))
	addMonitorRoutes(ws, monitor)
	return ws
}
