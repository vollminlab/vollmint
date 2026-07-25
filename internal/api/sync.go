package api

import (
	"log"
	"net/http"

	"github.com/vollminlab/vollmint/internal/store"
)

func (s *Server) registerSync() {
	s.mux.HandleFunc("GET /api/sync/status", s.handleSyncStatus)
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.SyncStatus(r.Context(), 20)
	if err != nil {
		log.Printf("sync status: %v", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	if runs == nil {
		runs = []store.SyncRun{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}
