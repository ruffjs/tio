package tio

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/emicklei/go-restful/v3"
)

var (
	//go:embed "config.default.yaml"
	DefaultConfigYaml []byte

	//go:embed "api/swagger_ui/*"
	swagFS embed.FS

	//go:embed "web/dist/*"
	webFS embed.FS
)

func RouteSwagger(container *restful.Container) {
	d, _ := fs.Sub(swagFS, "api/swagger_ui")
	container.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(d))))
}

func RouteWeb(container *restful.Container) {
	d, _ := fs.Sub(webFS, "web/dist")

	h := http.StripPrefix("/web/", http.FileServer(http.FS(d)))
	container.Handle("/web/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set cache header
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Cache-Control", "public, max-age=31536000")
		}
		h.ServeHTTP(w, r)
	}))

	container.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web", http.StatusFound)
	}))
}
