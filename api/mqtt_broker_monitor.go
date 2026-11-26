package api

import (
	"errors"
	"net/http"
	"strconv"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"ruff.io/tio/broker"
	rest "ruff.io/tio/pkg/restapi"
)

// BrokerMonitor is the read-only view of the embedded NATS broker.
type BrokerMonitor interface {
	Overview() broker.Overview
	Connections(broker.ConnQuery) (broker.ConnPage, error)
	Connection(cid uint64) (broker.ConnDetail, error)
}

func addMonitorRoutes(ws *restful.WebService, monitor BrokerMonitor) {
	ws.Route(ws.GET("/monitor/overview").
		To(func(req *restful.Request, resp *restful.Response) {
			if monitor == nil {
				sendBrokerErr(resp, http.StatusServiceUnavailable, "broker monitoring is not available")
				return
			}
			rest.SendRespOK(resp, monitor.Overview())
		}).
		Operation("broker-overview").
		Doc("Embedded NATS node overview and the last hour of message/byte rates").
		Metadata(restfulspec.KeyOpenAPITags, []string{"mqttBroker"}).
		Returns(http.StatusOK, "OK", rest.RespOK(broker.Overview{})))

	ws.Route(ws.GET("/monitor/connections").
		To(func(req *restful.Request, resp *restful.Response) {
			if monitor == nil {
				sendBrokerErr(resp, http.StatusServiceUnavailable, "broker monitoring is not available")
				return
			}
			q, err := parseConnQuery(req)
			if err != nil {
				sendBrokerErr(resp, http.StatusBadRequest, err.Error())
				return
			}
			page, err := monitor.Connections(q)
			if err != nil {
				sendBrokerErr(resp, http.StatusInternalServerError, "failed to read broker connections")
				return
			}
			rest.SendRespOK(resp, page)
		}).
		Operation("broker-connections").
		Doc("Online connections of the current node, filtered, sorted and paged by NATS").
		Metadata(restfulspec.KeyOpenAPITags, []string{"mqttBroker"}).
		Param(ws.QueryParameter("clientId", "Exact MQTT client ID").DataType("string")).
		Param(ws.QueryParameter("sort", "NATS connection sort: cid, start, subs, pending, msgs_to, msgs_from, bytes_to, bytes_from, last, idle, uptime or rtt").DataType("string")).
		Param(ws.QueryParameter("page", "Page number, starting at 1").DataType("integer")).
		Param(ws.QueryParameter("pageSize", "Page size, at most 200").DataType("integer")).
		Returns(http.StatusOK, "OK", rest.RespOK(broker.ConnPage{})))

	ws.Route(ws.GET("/monitor/connections/{cid}").
		To(func(req *restful.Request, resp *restful.Response) {
			if monitor == nil {
				sendBrokerErr(resp, http.StatusServiceUnavailable, "broker monitoring is not available")
				return
			}
			cid, err := strconv.ParseUint(req.PathParameter("cid"), 10, 64)
			if err != nil {
				sendBrokerErr(resp, http.StatusBadRequest, "invalid connection ID")
				return
			}
			detail, err := monitor.Connection(cid)
			if errors.Is(err, broker.ErrConnectionNotFound) {
				sendBrokerErr(resp, http.StatusNotFound, "connection closed")
				return
			}
			if err != nil {
				sendBrokerErr(resp, http.StatusInternalServerError, "failed to read broker connection")
				return
			}
			rest.SendRespOK(resp, detail)
		}).
		Operation("broker-connection").
		Doc("Subscriptions of one online connection").
		Metadata(restfulspec.KeyOpenAPITags, []string{"mqttBroker"}).
		Param(ws.PathParameter("cid", "NATS connection ID").DataType("integer")).
		Returns(http.StatusOK, "OK", rest.RespOK(broker.ConnDetail{})))

}

func parseConnQuery(req *restful.Request) (broker.ConnQuery, error) {
	q := broker.ConnQuery{
		ClientID: req.QueryParameter("clientId"),
		Sort:     req.QueryParameter("sort"),
	}
	if q.Sort == "" {
		q.Sort = "subs"
	}
	if !broker.ValidSort(q.Sort) {
		return q, errors.New("invalid sort field")
	}
	var err error
	if v := req.QueryParameter("page"); v != "" {
		if q.Page, err = strconv.Atoi(v); err != nil || q.Page < 1 {
			return q, errors.New("invalid page")
		}
	}
	if v := req.QueryParameter("pageSize"); v != "" {
		if q.PageSize, err = strconv.Atoi(v); err != nil || q.PageSize < 1 || q.PageSize > broker.MaxPageSize {
			return q, errors.New("invalid page size")
		}
	}
	return q, nil
}

func sendBrokerErr(resp *restful.Response, status int, message string) {
	rest.SendResp(resp, status, rest.Resp[any]{Code: status, Message: message})
}
