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

func (s *Server) registerCategories() {
	s.mux.HandleFunc("GET /api/categories", s.handleListCategories)
	s.mux.HandleFunc("POST /api/categories", s.handleCreateCategory)
	s.mux.HandleFunc("PATCH /api/categories/{id}", s.handlePatchCategory)
}

func validKind(k string) bool {
	switch k {
	case "spend", "income", "transfer", "savings":
		return true
	}
	return false
}

func (s *Server) handleListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.ListCategories(r.Context())
	if err != nil {
		log.Printf("list categories: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if cats == nil {
		cats = []store.Category{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}

func (s *Server) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		IsVice bool   `json:"is_vice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Kind == "" {
		body.Kind = "spend"
	}
	if !validKind(body.Kind) {
		writeErr(w, http.StatusBadRequest, "kind must be spend|income|transfer|savings")
		return
	}
	id, err := s.store.CreateCategory(r.Context(), body.Name, body.Kind, body.IsVice)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeErr(w, http.StatusBadRequest, "category name already exists")
			return
		}
		log.Printf("create category: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handlePatchCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Kind   *string `json:"kind"`
		IsVice *bool   `json:"is_vice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Kind != nil && !validKind(*body.Kind) {
		writeErr(w, http.StatusBadRequest, "kind must be spend|income|transfer|savings")
		return
	}
	err = s.store.UpdateCategory(r.Context(), id, store.CategoryPatch{
		Name: body.Name, Kind: body.Kind, IsVice: body.IsVice,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeErr(w, http.StatusBadRequest, "category name already exists")
			return
		}
		log.Printf("patch category: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
