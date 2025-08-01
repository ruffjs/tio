package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
)

func LoggingMiddleware(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	t := time.Now()
	chain.ProcessFilter(req, resp)
	slog.Info("Request",
		slog.String("method", req.Request.Method),
		slog.String("uri", req.Request.RequestURI),
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
