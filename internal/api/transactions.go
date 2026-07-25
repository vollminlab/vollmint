package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/vollminlab/vollmint/internal/store"
)

var monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)

// validView reports whether v is an accepted view filter.
func validView(v string) bool {
	switch v {
	case "scott", "nikki", "joint", "household":
		return true
	}
	return false
}

func (s *Server) registerTransactions() {
	s.mux.HandleFunc("GET /api/transactions", s.handleListTransactions)
	s.mux.HandleFunc("PATCH /api/transactions/{id}", s.handlePatchTransaction)
}

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	view := q.Get("view")
	if view == "" {
		view = "household"
	}
	if !validView(view) {
		writeErr(w, http.StatusBadRequest, "invalid view")
		return
	}
	month := q.Get("month")
	if month != "" {
		if !monthRe.MatchString(month) {
			writeErr(w, http.StatusBadRequest, "month must be YYYY-MM")
			return
		}
		if _, err := time.Parse("2006-01", month); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid month")
			return
		}
	}
	f := store.TxnFilter{
		View:          view,
		Month:         month,
		AccountID:     q.Get("account"),
		Query:         q.Get("q"),
		Uncategorized: q.Get("uncategorized") == "true",
	}
	if c := q.Get("category"); c != "" {
		id, err := strconv.Atoi(c)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "category must be an integer")
			return
		}
		f.CategoryID = &id
	}
	rows, err := s.store.ListTransactions(r.Context(), f)
	if err != nil {
		log.Printf("list transactions: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if rows == nil {
		rows = []store.TxnRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": rows})
}

type txnPatchBody struct {
	CategoryID    *int    `json:"category_id"`
	OwnerOverride *string `json:"owner_override"`
}

func (s *Server) handlePatchTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "id must be an integer")
		return
	}
	var body txnPatchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	err = s.store.UpdateTransaction(r.Context(), id, store.TxnPatch{
		CategoryID:    body.CategoryID,
		OwnerOverride: body.OwnerOverride,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "transaction not found")
		return
	}
	if err != nil {
		log.Printf("patch transaction: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
