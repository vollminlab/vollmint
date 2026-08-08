// Package api implements vollmint's HTTP surface: the JSON REST API under
// /api, Prometheus /metrics, an unauthenticated /healthz, and the embedded
// React SPA for all other paths. Auth is handled upstream by Authentik
// forward-auth; this server trusts the proxy.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/vollminlab/vollmint/internal/store"
)

// Server holds dependencies and builds the HTTP handler.
type Server struct {
	store *store.Store
	mux   *http.ServeMux
}

// New constructs a Server. store may be nil for tests that only exercise
// routes which do not touch the database (e.g. /healthz).
func New(s *store.Store) *Server {
	srv := &Server{store: s, mux: http.NewServeMux()}
	srv.routes()
	return srv
}

// routes registers every route group. Each group lives in its own file.
func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	s.registerTransactions()
	s.registerSplits()
	s.registerForecast()
	s.registerInsights()
	s.registerSummary()
	s.registerCategories()
	s.registerRules()
	s.registerBudgets()
	s.registerRecurring()
	s.registerTrends()
	s.registerImports()
	s.registerSync()
	s.registerNetWorth()
	s.registerStatic() // must be last: it owns the catch-all "/"
}

// Handler returns the fully wrapped handler (recover + log middleware).
func (s *Server) Handler() http.Handler {
	return logMiddleware(recoverMiddleware(s.mux))
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d", r.Method, r.URL.Path, sw.status)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// writeJSON serializes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

// writeErr sends a JSON error envelope: {"error":"message"}.
func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
