package api

import (
	"log"
	"net/http"

	"github.com/vollminlab/vollmint/internal/report"
)

func (s *Server) registerForecast() {
	s.mux.HandleFunc("GET /api/forecast", s.handleGetForecast)
}

func (s *Server) handleGetForecast(w http.ResponseWriter, r *http.Request) {
	view, month, ok := requireViewMonth(w, r)
	if !ok {
		return
	}
	f, err := report.Forecast(r.Context(), s.store, view, month)
	if err != nil {
		log.Printf("forecast: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forecast": f})
}
