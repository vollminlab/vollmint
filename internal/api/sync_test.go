package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vollminlab/vollmint/internal/store"
)

func TestGetSyncStatus(t *testing.T) {
	s := testStore(t)
	if _, err := s.Pool.Exec(context.Background(),
		`INSERT INTO sync_runs (kind,status,rows_upserted) VALUES ('simplefin','ok',3)`); err != nil {
		t.Fatal(err)
	}
	srv := New(s)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/sync/status", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "simplefin") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetSyncStatusEmpty(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/sync/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		Runs []store.SyncRun `json:"runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Runs == nil || len(body.Runs) != 0 {
		t.Fatalf("runs = %+v, want empty non-nil array", body.Runs)
	}
}
