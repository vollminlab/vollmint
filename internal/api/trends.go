package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/vollminlab/vollmint/internal/report"
)

func (s *Server) registerTrends() {
	s.mux.HandleFunc("GET /api/trends", s.handleTrends)
}

func (s *Server) handleTrends(w http.ResponseWriter, r *http.Request) {
	view, month, ok := requireViewMonth(w, r)
	if !ok {
		return
	}
	months := 12
	if raw := r.URL.Query().Get("months"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 60 {
			writeErr(w, http.StatusBadRequest, "months must be an integer between 1 and 60")
			return
		}
		months = n
	}
	points, err := report.MonthlyFlow(r.Context(), s.store, view, month, months)
	if err != nil {
		log.Printf("trends: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if points == nil {
		points = []report.MonthFlow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"trends": points})
}
