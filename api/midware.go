package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
)

func LoggingMiddleware(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	t := time.Now()
	chain.ProcessFilter(req, resp)
	route := req.SelectedRoutePath()
	if route == "" {
		route = req.Request.URL.Path
	}
	slog.Info("Request",
		slog.String("method", req.Request.Method),
		slog.String("uri", route),
		slog.Int("status", resp.StatusCode()),
		slog.Int64("duration_ms", time.Since(t).Milliseconds()))
}

func BasicAuthMiddleware(user, pass string) restful.FilterFunction {
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		u, p, ok := req.Request.BasicAuth()
		if !ok || u != user || p != pass {
			resp.WriteHeader(http.StatusUnauthorized)
			resp.Write([]byte("Unauthorized"))
			return
		}
		chain.ProcessFilter(req, resp)
	}
}

const (
	DefaultBodyLimit = 1 << 20 // 1 MiB
	BulkBodyLimit    = 8 << 20 // 8 MiB: batch thing import and full rule config
)

// BodyLimitMiddleware caps request bodies. Two routes legitimately carry large
// payloads, so they get their own limit instead of raising the global one.
func BodyLimitMiddleware() restful.FilterFunction {
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		limit := int64(DefaultBodyLimit)
		p, m := req.Request.URL.Path, req.Request.Method
		isBulk := (m == http.MethodPost && strings.HasSuffix(p, "/things/batch")) ||
			(m == http.MethodPut && strings.HasSuffix(p, "/rules/config"))
		if isBulk {
			limit = BulkBodyLimit
		}
		if req.Request.Body != nil {
			req.Request.Body = http.MaxBytesReader(resp, req.Request.Body, limit)
		}
		chain.ProcessFilter(req, resp)
	}
}
