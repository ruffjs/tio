package api_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	restful "github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/db/mock"
	"ruff.io/tio/shadow"
	shadowapi "ruff.io/tio/shadow/api"
	shadowmock "ruff.io/tio/shadow/mock"
	"ruff.io/tio/shadow/wire"
)

func newShadowServer(t *testing.T) (*httptest.Server, shadow.Service) {
	db := mock.NewSqliteConnTest()
	require.NoError(t, db.AutoMigrate(&shadow.Entity{}, &shadow.ConnStatusEntity{}))
	svc := wire.InitSvc(db, shadowmock.NewConnectivity(), shadow.Config{})

	ctx := context.Background()
	ws := new(restful.WebService)
	ws.Path("/api/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	ws.Route(ws.PATCH("/things/{id}/shadow/desired").To(shadowapi.PatchDesiredStateHandler(ctx, svc)))
	ws.Route(ws.PUT("/things/{id}/shadow/tags").To(shadowapi.SetTagsHandler(ctx, svc)))

	c := restful.NewContainer()
	c.Add(ws)
	return httptest.NewServer(c), svc
}

func doJSON(t *testing.T, svr *httptest.Server, method, path string, body any) int {
	b, err := json.Marshal(body, json.Deterministic(true))
	require.NoError(t, err)
	req, err := http.NewRequest(method, svr.URL+path, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := svr.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	t.Logf("%s %s -> %d %s", method, path, resp.StatusCode, buf.String())
	return resp.StatusCode
}

func TestShadowAPIVersionConflictReturns409(t *testing.T) {
	svr, svc := newShadowServer(t)
	defer svr.Close()

	thingId := fmt.Sprintf("api-409-%d", time.Now().UnixNano())
	_, err := svc.Create(context.Background(), thingId)
	require.NoError(t, err)

	desired := map[string]any{
		"clientToken": "token-1",
		"version":     1,
		"state":       map[string]any{"desired": map[string]any{"a": 1}},
	}
	require.Equal(t, http.StatusOK, doJSON(t, svr, http.MethodPatch, "/api/v1/things/"+thingId+"/shadow/desired", desired))
	require.Equal(t, http.StatusConflict, doJSON(t, svr, http.MethodPatch, "/api/v1/things/"+thingId+"/shadow/desired", desired))

	tags := map[string]any{"version": 1, "tags": map[string]any{"zone": "x"}}
	require.Equal(t, http.StatusConflict, doJSON(t, svr, http.MethodPut, "/api/v1/things/"+thingId+"/shadow/tags", tags))
}
