package api_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful/v3"
	"github.com/stretchr/testify/require"
	"ruff.io/tio/api"
)

func TestBodyLimitMiddleware(t *testing.T) {
	ws := new(restful.WebService)
	ws.Path("/api/v1")
	read := func(r *restful.Request, w *restful.Response) {
		b, err := io.ReadAll(r.Request.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte{byte(len(b) / 1024 / 1024)})
	}
	ws.Route(ws.POST("/things/batch").To(read))
	ws.Route(ws.PUT("/rules/config").To(read))
	ws.Route(ws.POST("/things").To(read))

	c := restful.NewContainer()
	c.Filter(api.BodyLimitMiddleware())
	c.Add(ws)
	svr := httptest.NewServer(c)
	defer svr.Close()

	post := func(path string, n int) int {
		req, err := http.NewRequest(http.MethodPost, svr.URL+path, bytes.NewReader(make([]byte, n)))
		if path == "/api/v1/rules/config" {
			req.Method = http.MethodPut
		}
		require.NoError(t, err)
		resp, err := svr.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}

	require.Equal(t, http.StatusOK, post("/api/v1/things", 512<<10))
	require.Equal(t, http.StatusBadRequest, post("/api/v1/things", 2<<20))
	require.Equal(t, http.StatusOK, post("/api/v1/things/batch", 6<<20))
	require.Equal(t, http.StatusBadRequest, post("/api/v1/things/batch", 9<<20))
	require.Equal(t, http.StatusOK, post("/api/v1/rules/config", 6<<20))
	require.Equal(t, http.StatusBadRequest, post("/api/v1/rules/config", 9<<20))
}
