package api

import (
	"log"
	"net/http"
	"time"

	"github.com/vollminlab/vollmint/internal/report"
)

func (s *Server) registerSummary() {
	s.mux.HandleFunc("GET /api/summary", s.handleSummary)
}

// requireViewMonth parses and validates the shared view+month query params.
// It writes a 400 and returns ok=false on any problem.
func requireViewMonth(w http.ResponseWriter, r *http.Request) (view, month string, ok bool) {
	view = r.URL.Query().Get("view")
	if view == "" {
		view = "household"
	}
	if !validView(view) {
		writeErr(w, http.StatusBadRequest, "invalid view")
		return "", "", false
	}
	month = r.URL.Query().Get("month")
	if !monthRe.MatchString(month) {
		writeErr(w, http.StatusBadRequest, "month is required and must be YYYY-MM")
		return "", "", false
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		writeErr(w, http.StatusBadRequest, "month is required and must be YYYY-MM")
		return "", "", false
	}
	return view, month, true
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	view, month, ok := requireViewMonth(w, r)
	if !ok {
		return
	}
	sum, err := report.Summary(r.Context(), s.store, view, month)
	if err != nil {
		log.Printf("summary: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	cats, err := report.SpendByCategory(r.Context(), s.store, view, month)
	if err != nil {
		log.Printf("summary: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if cats == nil {
		cats = []report.CategorySpend{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": sum, "categories": cats})
}
