package api

import (
	"time"

	"github.com/emicklei/go-restful/v3"
	"ruff.io/tio/pkg/log"
	rest "ruff.io/tio/pkg/restapi"
)

func LoggingMiddleware(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	t := time.Now()
	chain.ProcessFilter(req, resp)
	log.Infof("Request \"%s %s\" %d %dms",
		req.Request.Method, req.Request.RequestURI, resp.StatusCode(), time.Since(t).Milliseconds())
}

func BasicAuthMiddleware(user, pass string) restful.FilterFunction {
	return func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		u, p, ok := req.Request.BasicAuth()
		if !ok || u != user || p != pass {
			rest.SendResp(resp, 401, rest.Resp[any]{Code: 401, Message: "Unauthorized"})
			return
		}
		chain.ProcessFilter(req, resp)
	}
}
