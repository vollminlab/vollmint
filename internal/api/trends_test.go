package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTrends(t *testing.T) {
	s := testStore(t)
	seedTxn(t, s, "ally-s", "scott", "tr1", "2026-06-10", "-50.00", "WHOLE FOODS")
	seedTxn(t, s, "ally-s", "scott", "tr2", "2026-07-05", "-25.00", "WHOLE FOODS")
	srv := New(s)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/trends?view=household&month=2026-07&months=3", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Trends []struct {
			Month string `json:"month"`
			In    string `json:"in"`
			Out   string `json:"out"`
		} `json:"trends"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Trends) != 3 {
		t.Fatalf("got %d trend rows, want 3: %+v", len(body.Trends), body.Trends)
	}
	if body.Trends[0].Month != "2026-05" || body.Trends[2].Month != "2026-07" {
		t.Errorf("window = %s..%s, want 2026-05..2026-07", body.Trends[0].Month, body.Trends[2].Month)
	}
	if body.Trends[1].Out != "50.00" || body.Trends[2].Out != "25.00" {
		t.Errorf("trends = %+v", body.Trends)
	}
}

func TestGetTrendsDefaultsTo12Months(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/trends?view=household&month=2026-07", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Trends []struct {
			Month string `json:"month"`
		} `json:"trends"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Trends) != 12 {
		t.Errorf("got %d trend rows, want 12 (default window)", len(body.Trends))
	}
}

func TestGetTrendsBadMonths(t *testing.T) {
	srv := New(testStore(t))
	for _, months := range []string{"0", "61", "abc", "-3"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/trends?view=household&month=2026-07&months="+months, nil)
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("months=%s: status=%d, want 400", months, rec.Code)
		}
	}
}

func TestGetTrendsBadView(t *testing.T) {
	srv := New(testStore(t))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/trends?view=alien&month=2026-07", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (invalid view)", rec.Code)
	}
}
