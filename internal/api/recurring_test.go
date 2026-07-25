package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRecurring(t *testing.T) {
	s := testStore(t)
	ctx := nil0()
	seedTxn(t, s, "ally-s", "scott", "nf1", "2026-05-10", "-15.99", "NETFLIX")
	seedTxn(t, s, "ally-s", "scott", "nf2", "2026-06-10", "-15.99", "NETFLIX")
	seedTxn(t, s, "ally-s", "scott", "nf3", "2026-07-10", "-15.99", "NETFLIX")
	// seedTxn sets Payee to desc by default, but update to ensure consistency with report tests
	if _, err := s.Pool.Exec(ctx, `UPDATE transactions SET payee='NETFLIX' WHERE external_id IN ('nf1','nf2','nf3')`); err != nil {
		t.Fatal(err)
	}
	srv := New(s)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recurring?view=household&month=2026-07", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Recurring []struct {
			Payee     string `json:"payee"`
			AvgAmount string `json:"avg_amount"`
		} `json:"recurring"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Recurring) != 1 || body.Recurring[0].Payee != "NETFLIX" {
		t.Errorf("recurring = %+v", body.Recurring)
	}
	if body.Recurring[0].AvgAmount != "15.99" {
		t.Errorf("avg_amount = %s, want 15.99", body.Recurring[0].AvgAmount)
	}
}

func TestGetRecurringBadView(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recurring?view=alien&month=2026-07", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (invalid view)", rec.Code)
	}
}

func TestGetRecurringBadMonth(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recurring?view=household&month=2026-13", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (bad month)", rec.Code)
	}
}

func TestGetRecurringMissingParams(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recurring", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (month required)", rec.Code)
	}
}
