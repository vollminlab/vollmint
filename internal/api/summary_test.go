package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSummary(t *testing.T) {
	s := testStore(t)
	seedTxn(t, s, "ally-s", "scott", "sp1", "2026-07-05", "-100.00", "Groceries")
	// categorize it so spend-by-category has a row
	_, _ = s.Pool.Exec(nil0(),
		`UPDATE transactions SET category_id=(SELECT id FROM categories WHERE name='Groceries') WHERE external_id='sp1'`)
	srv := New(s)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/summary?view=household&month=2026-07", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Summary struct {
			Out string `json:"out"`
		} `json:"summary"`
		Categories []struct {
			Category string `json:"category"`
			Spent    string `json:"spent"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.Out != "100.00" {
		t.Errorf("summary.out = %q, want 100.00", body.Summary.Out)
	}
	if len(body.Categories) != 1 || body.Categories[0].Category != "Groceries" {
		t.Errorf("categories = %+v", body.Categories)
	}
}

func TestGetSummaryRequiresMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/summary?view=household", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (month required)", rec.Code)
	}
}
