package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vollminlab/vollmint/internal/ingest"
	"github.com/vollminlab/vollmint/internal/store"
)

func (s *Server) registerRules() {
	s.mux.HandleFunc("GET /api/rules", s.handleListRules)
	s.mux.HandleFunc("POST /api/rules", s.handleCreateRule)
	s.mux.HandleFunc("DELETE /api/rules/{id}", s.handleDeleteRule)
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.store.ListRules(r.Context())
	if err != nil {
		log.Printf("list rules: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if rules == nil {
		rules = []store.Rule{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Priority   int    `json:"priority"`
		MatchType  string `json:"match_type"`
		Pattern    string `json:"pattern"`
		CategoryID int    `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Pattern == "" || body.CategoryID == 0 {
		writeErr(w, http.StatusBadRequest, "pattern and category_id are required")
		return
	}
	if body.MatchType == "" {
		body.MatchType = "substring"
	}
	if body.MatchType != "substring" && body.MatchType != "regex" {
		writeErr(w, http.StatusBadRequest, "match_type must be substring|regex")
		return
	}
	if body.MatchType == "regex" {
		if _, err := regexp.Compile(body.Pattern); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid regex pattern")
			return
		}
	}
	id, err := s.store.CreateRule(r.Context(), body.Priority, body.MatchType, body.Pattern, body.CategoryID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514") {
			writeErr(w, http.StatusBadRequest, "invalid category_id")
			return
		}
		log.Printf("create rule: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Re-run rules over uncategorized history so the new rule takes effect now.
	n, err := ingest.ApplyRules(r.Context(), s.store)
	if err != nil {
		log.Printf("apply rules after create: %v", err)
		writeErr(w, http.StatusInternalServerError, "rule created but re-apply failed")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "recategorized": n})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	err = s.store.DeleteRule(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		log.Printf("delete rule: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
