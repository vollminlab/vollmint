package api

import (
	"log"
	"net/http"
	"time"

	"github.com/vollminlab/vollmint/internal/report"
)

func (s *Server) registerInsights() {
	s.mux.HandleFunc("GET /api/insights", s.handleGetInsights)
}

func (s *Server) handleGetInsights(w http.ResponseWriter, r *http.Request) {
	view, month, ok := requireViewMonth(w, r)
	if !ok {
		return
	}
	items, err := report.Insights(r.Context(), s.store, view, month, time.Now())
	if err != nil {
		log.Printf("insights: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if items == nil {
		items = []report.Insight{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"insights": items})
}
