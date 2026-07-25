package api

import (
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vollminlab/vollmint/web"
)

// registerStatic wires /metrics and the SPA. The SPA is served from the
// embedded web/dist; any non-/api, non-file path falls back to index.html so
// client-side routing works. /api/* is never served the SPA — it 404s cleanly.
func (s *Server) registerStatic() {
	s.mux.Handle("GET /metrics", promhttp.Handler())

	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic("embed web/dist: " + err.Error()) // build-time guarantee
	}
	fileServer := http.FileServer(http.FS(dist))

	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// /api/* that reached here matched no API route → 404 (never SPA).
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		// If the requested path exists as an embedded file, serve it.
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			serveIndex(w, dist)
			return
		}
		if _, err := fs.Stat(dist, p); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Otherwise it's a client-side route → serve index.html.
		serveIndex(w, dist)
	})
}

func serveIndex(w http.ResponseWriter, dist fs.FS) {
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		log.Printf("serve index: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
