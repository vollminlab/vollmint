package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vollminlab/vollmint/internal/store"
)

func (s *Server) registerBudgets() {
	s.mux.HandleFunc("GET /api/budgets", s.handleGetBudgets)
	s.mux.HandleFunc("PUT /api/budgets", s.handlePutBudgets)
}

func (s *Server) handleGetBudgets(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if !monthRe.MatchString(month) {
		writeErr(w, http.StatusBadRequest, "month is required and must be YYYY-MM")
		return
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		writeErr(w, http.StatusBadRequest, "month is required and must be YYYY-MM")
		return
	}

	items, err := s.store.GetBudgets(r.Context(), month)
	if err != nil {
		log.Printf("get budgets: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if items == nil {
		items = []store.BudgetItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"budgets": items})
}

func (s *Server) handlePutBudgets(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if !monthRe.MatchString(month) {
		writeErr(w, http.StatusBadRequest, "month is required and must be YYYY-MM")
		return
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		writeErr(w, http.StatusBadRequest, "month is required and must be YYYY-MM")
		return
	}

	var req struct {
		Budgets []store.BudgetItem `json:"budgets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	err := s.store.PutBudgets(r.Context(), month, req.Budgets)
	if err != nil {
		if errors.Is(err, store.ErrInvalidAmount) {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // FK error
				writeErr(w, http.StatusBadRequest, "invalid category_id")
				return
			case "23505": // unique constraint error
				writeErr(w, http.StatusBadRequest, "duplicate category in budgets")
				return
			}
		}
		log.Printf("put budgets: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
