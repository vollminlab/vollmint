package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/vollminlab/vollmint/internal/report"
	"github.com/vollminlab/vollmint/internal/store"
)

// rangeDays maps the networth range param to a day count; 0 = all history.
var rangeDays = map[string]int{"1m": 30, "3m": 91, "6m": 182, "1y": 365, "all": 0}

func (s *Server) registerNetWorth() {
	s.mux.HandleFunc("GET /api/networth", s.handleNetWorth)
	s.mux.HandleFunc("POST /api/accounts/manual", s.handleCreateManualAccount)
	s.mux.HandleFunc("PUT /api/accounts/{id}/balance", s.handleUpdateManualBalance)
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

func (s *Server) handleCreateManualAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Owner   string `json:"owner"`
		Balance string `json:"balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if store.ManualAccountID(name) == "manual-" {
		writeErr(w, http.StatusBadRequest, "name must contain a letter or number")
		return
	}
	switch body.Owner {
	case "scott", "nikki", "joint":
	default:
		writeErr(w, http.StatusBadRequest, "owner must be scott, nikki, or joint")
		return
	}
	id, err := s.store.CreateManualAccount(r.Context(), name, body.Owner, body.Balance)
	switch {
	case errors.Is(err, store.ErrInvalidAmount):
		writeErr(w, http.StatusBadRequest, "balance must be a decimal amount")
	case errors.Is(err, store.ErrConflict):
		writeErr(w, http.StatusConflict, "an account with this name already exists")
	case err != nil:
		log.Printf("create manual account: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
	default:
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

func (s *Server) handleUpdateManualBalance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Balance string `json:"balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	err := s.store.UpdateManualBalance(r.Context(), r.PathValue("id"), body.Balance)
	switch {
	case errors.Is(err, store.ErrInvalidAmount):
		writeErr(w, http.StatusBadRequest, "balance must be a decimal amount")
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "account not found")
	case errors.Is(err, store.ErrNotManual):
		writeErr(w, http.StatusBadRequest, "balance edits are only allowed on manual accounts")
	case err != nil:
		log.Printf("update manual balance: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
	default:
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
