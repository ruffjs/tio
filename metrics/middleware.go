package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful/v3"
)

// Middleware returns a go-restful filter function that collects HTTP metrics
func Middleware(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	start := time.Now()

	// Get route pattern
	route := req.Request.URL.Path
	if route == "" {
		route = "unknown"
	}

	// Record request size if available
	if req.Request.ContentLength > 0 {
		HTTPRequestSize.WithLabelValues(req.Request.Method, route).Observe(float64(req.Request.ContentLength))
	}

	// Process the request
	chain.ProcessFilter(req, resp)

	// Record metrics
	duration := time.Since(start).Seconds()
	statusCode := http.StatusText(resp.StatusCode())
	if statusCode == "" {
		statusCode = "unknown"
	}
	statusCodeLabel := fmt.Sprintf("%d", resp.StatusCode())

	HTTPRequestDuration.WithLabelValues(req.Request.Method, route, statusCodeLabel).Observe(duration)
	HTTPRequestsTotal.WithLabelValues(req.Request.Method, route, statusCodeLabel).Inc()

	// Record response size if available
	if resp.ContentLength() > 0 {
		HTTPResponseSize.WithLabelValues(req.Request.Method, route).Observe(float64(resp.ContentLength()))
	}
}
