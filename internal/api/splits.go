package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/vollminlab/vollmint/internal/store"
)

func (s *Server) registerSplits() {
	s.mux.HandleFunc("PUT /api/transactions/{id}/splits", s.handlePutSplits)
	s.mux.HandleFunc("DELETE /api/transactions/{id}/splits", s.handleDeleteSplits)
}

type putSplitsBody struct {
	Splits []store.SplitInput `json:"splits"`
}

func (s *Server) handlePutSplits(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	var body putSplitsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	err = s.store.ReplaceSplits(r.Context(), id, body.Splits)
	switch {
	case err == nil:
		// fall through to re-fetch below
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "transaction not found")
		return
	case errors.Is(err, store.ErrSplitTooFew),
		errors.Is(err, store.ErrSplitParent),
		errors.Is(err, store.ErrSplitSign),
		errors.Is(err, store.ErrInvalidAmount):
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	default:
		var sumErr *store.SplitSumError
		if errors.As(err, &sumErr) {
			writeErr(w, http.StatusBadRequest, sumErr.Error())
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			writeErr(w, http.StatusBadRequest, "unknown category_id")
			return
		}
		log.Printf("put splits: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	txn, err := s.store.GetTransaction(r.Context(), id)
	if err != nil {
		log.Printf("get transaction after split: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transaction": txn})
}

func (s *Server) handleDeleteSplits(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	if err := s.store.DeleteSplits(r.Context(), id); err != nil {
		log.Printf("delete splits: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
