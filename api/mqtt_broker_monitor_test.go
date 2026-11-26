package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/broker"
)

type fakeBrokerMonitor struct {
	overview broker.Overview
	page     broker.ConnPage
	detail   broker.ConnDetail
	err      error
}

func (f fakeBrokerMonitor) Overview() broker.Overview { return f.overview }

func (f fakeBrokerMonitor) Connections(q broker.ConnQuery) (broker.ConnPage, error) {
	return f.page, f.err
}

func (f fakeBrokerMonitor) Connection(uint64) (broker.ConnDetail, error) {
	return f.detail, f.err
}

func newMonitorContainer(m BrokerMonitor) *restful.Container {
	c := restful.NewContainer()
	c.Add(MQTTBrokerService(nil, nil, m))
	return c
}

func doGet(t *testing.T, c *restful.Container, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, req)
	return rec
}

func TestMonitorOverview(t *testing.T) {
	c := newMonitorContainer(fakeBrokerMonitor{overview: broker.Overview{
		Node: "node-1", Connections: 3,
		Consumer: broker.ConsumerUsage{Stream: broker.StreamName, State: "ok", Usage: 1, Limit: 10, LimitState: "limited"},
	}})

	rec := doGet(t, c, "/api/v1/mqttBroker/monitor/overview")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Code int             `json:"code"`
		Data broker.Overview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 200, body.Code)
	assert.Equal(t, "node-1", body.Data.Node)
	assert.Equal(t, 3, body.Data.Connections)
}

func TestMonitorConnectionsValidatesQuery(t *testing.T) {
	c := newMonitorContainer(fakeBrokerMonitor{page: broker.ConnPage{Total: 2, Items: []broker.ConnItem{}}})

	rec := doGet(t, c, "/api/v1/mqttBroker/monitor/connections?sort=msgs_to&page=1&pageSize=20")
	require.Equal(t, http.StatusOK, rec.Code)

	rec = doGet(t, c, "/api/v1/mqttBroker/monitor/connections?sort=bogus")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doGet(t, c, "/api/v1/mqttBroker/monitor/connections?sort=stop")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doGet(t, c, "/api/v1/mqttBroker/monitor/connections?page=0")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doGet(t, c, "/api/v1/mqttBroker/monitor/connections?pageSize=201")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMonitorConnection(t *testing.T) {
	c := newMonitorContainer(fakeBrokerMonitor{
		detail: broker.ConnDetail{CID: 7, ClientID: "dev-7", Subscriptions: []string{"a"}},
	})
	rec := doGet(t, c, "/api/v1/mqttBroker/monitor/connections/7")
	require.Equal(t, http.StatusOK, rec.Code)

	rec = doGet(t, c, "/api/v1/mqttBroker/monitor/connections/abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	c = newMonitorContainer(fakeBrokerMonitor{err: broker.ErrConnectionNotFound})
	rec = doGet(t, c, "/api/v1/mqttBroker/monitor/connections/8")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
