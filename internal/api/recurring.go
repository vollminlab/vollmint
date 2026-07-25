package api

import (
	"log"
	"net/http"

	"github.com/vollminlab/vollmint/internal/report"
)

func (s *Server) registerRecurring() {
	s.mux.HandleFunc("GET /api/recurring", s.handleRecurring)
}

func (s *Server) handleRecurring(w http.ResponseWriter, r *http.Request) {
	view, month, ok := requireViewMonth(w, r)
	if !ok {
		return
	}
	items, err := report.Recurring(r.Context(), s.store, view, month)
	if err != nil {
		log.Printf("recurring: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if items == nil {
		items = []report.RecurringItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"recurring": items})
}
