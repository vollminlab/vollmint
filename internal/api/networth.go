package api

import (
	"log"
	"net/http"

	"github.com/vollminlab/vollmint/internal/report"
)

// rangeDays maps the networth range param to a day count; 0 = all history.
var rangeDays = map[string]int{"1m": 30, "3m": 91, "6m": 182, "1y": 365, "all": 0}

func (s *Server) registerNetWorth() {
	s.mux.HandleFunc("GET /api/networth", s.handleNetWorth)
}

func (s *Server) handleNetWorth(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "household"
	}
	if !validView(view) {
		writeErr(w, http.StatusBadRequest, "invalid view")
		return
	}
	rng := r.URL.Query().Get("range")
	if rng == "" {
		rng = "3m"
	}
	days, ok := rangeDays[rng]
	if !ok {
		writeErr(w, http.StatusBadRequest, "range must be one of 1m, 3m, 6m, 1y, all")
		return
	}
	series, err := report.NetWorthSeries(r.Context(), s.store, view, days)
	if err != nil {
		log.Printf("networth series: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	accounts, err := report.NetWorthAccounts(r.Context(), s.store, view)
	if err != nil {
		log.Printf("networth accounts: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if series == nil {
		series = []report.NetWorthPoint{}
	}
	if accounts == nil {
		accounts = []report.NetWorthAccount{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": series, "accounts": accounts})
}
