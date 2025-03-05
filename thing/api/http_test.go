package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ruff.io/tio/db/mock"
	"ruff.io/tio/shadow"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	tmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/pkg/model"
	rest "ruff.io/tio/pkg/restapi"
	"ruff.io/tio/pkg/uuid"
	shadowMock "ruff.io/tio/shadow/mock"
	"ruff.io/tio/thing"
	"ruff.io/tio/thing/api"

	gapi "ruff.io/tio/api"
)

type mockShadowSvc struct {
	tmock.Mock
	shadow.Service
}

func (m *mockShadowSvc) NotifyCreated(thingId string, s shadow.ShadowWithEnable) {
	m.Called(thingId, s)
}

func (m *mockShadowSvc) NotifyDeleted(thingId string) {
	m.Called(thingId)
}

var connector = shadowMock.NewConnectivity()

func newServer() *httptest.Server {
	mkSs := new(mockShadowSvc)
	mkSs.On("NotifyCreated", tmock.Anything, tmock.Anything)
	mkSs.On("NotifyDeleted", tmock.Anything)

	conn := mock.NewSqliteConnTest()
	_ = conn.AutoMigrate(&thing.Entity{}, &shadow.Entity{}, &shadow.ConnStatusEntity{})
	repo := thing.NewThingRepo(conn)
	svc := thing.NewSvc(repo, uuid.New(), mkSs, connector)

	apiSvc := api.Service(context.Background(), svc)
	container := restful.NewContainer()
	container.ServeMux = http.NewServeMux()
	container.Add(apiSvc)
	container.Add(restfulspec.NewOpenAPIService(gapi.OpenapiConfig()))
	return httptest.NewServer(container)
}

var createThReq = api.CreateReq{}
var idProv = uuid.New()

func id() string {
	s, _ := idProv.ID()
	return s
}

func toBuf(t any) *bytes.Buffer {
	b, _ := json.Marshal(t)
	return bytes.NewBuffer(b)
}

func TestCreateHandler(t *testing.T) {
	t.Parallel()
	svr := newServer()
	defer svr.Close()

	doReq := func(r api.CreateReq) (*http.Response, error) {
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", svr.URL), toBuf(r))
		req.Header.Set("Content-Type", "application/json")
		resp, err := svr.Client().Do(req)
		require.NoError(t, err)
		return resp, err
	}

	decodeRes := func(resp *http.Response) (thing.Thing, error) {
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var resD rest.Resp[thing.Thing]
		err := json.NewDecoder(resp.Body).Decode(&resD)
		var rTh = resD.Data
		return rTh, err
	}

	t.Run("should request ok", func(t *testing.T) {
		th := createThReq
		th.ThingId = "some-id-xxx"
		resp, err := doReq(th)
		require.NoError(t, err)
		require.Equal(t, resp.StatusCode, http.StatusOK)
	})

	t.Run("wrong thingId should error", func(t *testing.T) {
		cases := []struct {
			id  string
			pwd string
		}{
			{"sdf$", ""}, {"#sd/df", ""}, {"&sdf", ""},
			// id or password length greater than 64
			{strings.Repeat("x", 65), ""},
			{strings.Repeat("x", 90), ""},
			{"sds", strings.Repeat("p", 65)},
			{"sds", strings.Repeat("p", 165)},
			{strings.Repeat("x", 90), strings.Repeat("p", 165)},
		}
		for _, c := range cases {
			th := createThReq
			th.ThingId = c.id
			th.Password = c.pwd
			resp, err := doReq(th)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		}
	})

	t.Run("should create ok", func(t *testing.T) {
		th := createThReq
		th.ThingId = id()
		resp, err := doReq(th)
		require.NoError(t, err)
		rTh, err := decodeRes(resp)
		require.NoError(t, err)

		require.Equal(t, th.ThingId, rTh.Id)
		require.True(t, rTh.Enabled, "thing should be enabled by default")
		require.Equal(t, thing.AuthTypePassword, rTh.AuthType, "auth type should be password by default")
	})

	t.Run("should create with password ok", func(t *testing.T) {
		th := createThReq
		th.ThingId = id()
		th.Password = id()

		resp, err := doReq(th)
		require.NoError(t, err)
		rTh, err := decodeRes(resp)
		require.NoError(t, err)

		require.Equal(t, th.ThingId, rTh.Id)
		require.Equal(t, thing.AuthTypePassword, rTh.AuthType, "auth type should be password by default")
		require.Equal(t, th.Password, rTh.AuthValue, "password should be equal with request")
	})
}

func TestCreateBatchHandler(t *testing.T) {
	t.Parallel()
	svr := newServer()
	defer svr.Close()

	vaildThing := api.CreateReq{"some-id-xxx", "password", true, nil}
	noPasswordThing := api.CreateReq{ThingId: "noPasswordThing"}

	doReq := func(r []api.CreateReq) (*http.Response, error) {
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things/batch", svr.URL), toBuf(r))
		req.Header.Set("Content-Type", "application/json")
		resp, err := svr.Client().Do(req)
		require.NoError(t, err)
		return resp, err
	}

	decodeRes := func(resp *http.Response) (api.CreateBatchResp, error) {
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var resD rest.Resp[api.CreateBatchResp]
		err := json.NewDecoder(resp.Body).Decode(&resD)
		var rTh = resD.Data
		return rTh, err
	}

	t.Run("should request ok", func(t *testing.T) {
		ths := []api.CreateReq{vaildThing, noPasswordThing}
		resp, err := doReq(ths)
		require.NoError(t, err)

		rTh, err := decodeRes(resp)
		require.NoError(t, err)

		require.Equal(t, resp.StatusCode, http.StatusOK)
		require.Equal(t, len(rTh.ValidList), len(ths))
	})

	t.Run("wrong thingId should error", func(t *testing.T) {
		ths := []api.CreateReq{}
		cases := []struct {
			id  string
			pwd string
		}{
			{"sdf$", ""}, {"#sd/df", ""}, {"&sdf", ""},
			// id or password length greater than 64
			{strings.Repeat("x", 65), ""},
			{strings.Repeat("x", 90), ""},
			{"sds", strings.Repeat("p", 65)},
			{"sds", strings.Repeat("p", 165)},
			{strings.Repeat("x", 90), strings.Repeat("p", 165)},
		}
		for _, c := range cases {
			th := createThReq
			th.ThingId = c.id
			th.Password = c.pwd
			ths = append(ths, th)
		}
		resp, err := doReq(ths)
		require.NoError(t, err)

		rTh, err := decodeRes(resp)
		require.NoError(t, err)
		require.Equal(t, len(rTh.InvalidList), len(cases))
	})
}

func TestQueryHandler(t *testing.T) {
	t.Parallel()
	svr := newServer()
	defer svr.Close()

	gwCount := rand.Intn(10)
	normalCount := rand.Intn(10)

	ths := []api.CreateReq{}
	for i := 0; i < gwCount; i++ {
		ths = append(ths, api.CreateReq{ThingId: id(), IsGateway: true})
	}
	for i := 0; i < normalCount; i++ {
		ths = append(ths, api.CreateReq{ThingId: id(), IsGateway: false})
	}

	for _, th := range ths {
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", svr.URL), toBuf(th))
		req.Header.Set("Content-Type", "application/json")
		client := svr.Client()
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	t.Run("should query ok", func(t *testing.T) {
		client := svr.Client()

		//query thing
		req, _ := http.NewRequest(http.MethodGet,
			fmt.Sprintf("%s/api/v1/things?withAuthValue=true&pageIndex=1&pageSize=20", svr.URL), toBuf(nil))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		var resD rest.Resp[model.PageData[thing.Thing]]
		err = json.NewDecoder(resp.Body).Decode(&resD)
		require.Equal(t, resD.Data.Total, int64(gwCount+normalCount))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		resD2 := rest.Resp[model.PageData[thing.Thing]]{}
		req, _ = http.NewRequest(http.MethodGet,
			fmt.Sprintf("%s/api/v1/things?withAuthValue=true&isGateway=true&pageIndex=1&pageSize=20", svr.URL), toBuf(nil))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		err = json.NewDecoder(resp.Body).Decode(&resD2)
		require.Equal(t, int64(gwCount), resD2.Data.Total)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Falsef(t, slices.ContainsFunc(resD2.Data.Content, func(th thing.Thing) bool {
			return !th.IsGateway
		}), "should all be gateway")
	})
}

func TestGetHandler(t *testing.T) {
	t.Parallel()
	svr := newServer()
	defer svr.Close()
	t.Run("should request ok", func(t *testing.T) {
		// create thing
		th := createThReq
		th.ThingId = id()
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", svr.URL), toBuf(th))
		req.Header.Set("Content-Type", "application/json")
		client := svr.Client()
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// query thing
		req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/things/%s", svr.URL, th.ThingId), toBuf(nil))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		var resD rest.Resp[thing.Thing]
		err = json.NewDecoder(resp.Body).Decode(&resD)
		require.Equal(t, resD.Data.Id, th.ThingId)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestDeleteHandler(t *testing.T) {
	t.Parallel()
	svr := newServer()
	defer svr.Close()
	t.Run("should delete ok", func(t *testing.T) {
		// create thing
		th := createThReq
		th.ThingId = id()
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", svr.URL), toBuf(th))
		req.Header.Set("Content-Type", "application/json")
		client := svr.Client()
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// delete thing

		connector.On("Close", th.ThingId).Return(nil)
		connector.On("Remove", th.ThingId).Return(nil)
		req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", svr.URL, th.ThingId), toBuf(nil))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		var resD rest.Resp[any]
		err = json.NewDecoder(resp.Body).Decode(&resD)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, http.StatusOK, resD.Code)

		// delete thing
		// repeat delete thing, got no error
		req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/things/%s", svr.URL, th.ThingId), toBuf(nil))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		var resD2 rest.Resp[any]
		err = json.NewDecoder(resp.Body).Decode(&resD2)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, http.StatusOK, resD2.Code)
	})
}

func TestBindHandler(t *testing.T) {
	t.Parallel()
	svr := newServer()
	defer svr.Close()

	// create thing
	gwId := "gw-1"
	thId := "th-1"
	th := api.CreateReq{
		ThingId: thId,
	}
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", svr.URL), toBuf(th))
	req.Header.Set("Content-Type", "application/json")
	client := svr.Client()
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	gw := api.CreateReq{
		ThingId:   gwId,
		IsGateway: true,
	}
	req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things", svr.URL), toBuf(gw))
	req.Header.Set("Content-Type", "application/json")
	client = svr.Client()
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	t.Run("should bind ok", func(t *testing.T) {
		body := thing.ThingBindReq{ThingIds: []string{th.ThingId}}
		req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things/%s/bind/", svr.URL, gw.ThingId), toBuf(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		var resD rest.Resp[any]
		err = json.NewDecoder(resp.Body).Decode(&resD)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, http.StatusOK, resD.Code)
	})

	t.Run("should unbind ok", func(t *testing.T) {
		body := thing.ThingBindReq{ThingIds: []string{th.ThingId}}
		req, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/things/%s/unbind", svr.URL, gw.ThingId), toBuf(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		require.NoError(t, err)
		var resD rest.Resp[any]
		err = json.NewDecoder(resp.Body).Decode(&resD)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, http.StatusOK, resD.Code)
	})
}
