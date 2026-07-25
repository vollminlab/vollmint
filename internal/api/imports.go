package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/vollminlab/vollmint/internal/ingest"
)

// maxUploadBytes caps a Venmo CSV upload. A 90-day export is a few hundred KB;
// 10 MiB is generous headroom and bounds memory.
const maxUploadBytes = 10 << 20

func (s *Server) registerImports() {
	s.mux.HandleFunc("POST /api/imports/venmo", s.handleVenmoUpload)
}

func (s *Server) handleVenmoUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "missing form field 'file'")
		return
	}
	defer file.Close()

	// The CSV is streamed through the parser and never persisted to disk.
	res, err := ingest.ImportVenmo(r.Context(), s.store, file)
	if err != nil {
		// Parse errors (wrapped with "parse venmo csv:") are client faults.
		if strings.Contains(err.Error(), "parse venmo csv:") {
			writeErr(w, http.StatusBadRequest, "import failed: "+err.Error())
		} else {
			log.Printf("venmo import: %v", err)
			writeErr(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"upserted":    res.Upserted,
		"categorized": res.Categorized,
		"paired":      res.Paired,
	})
}
