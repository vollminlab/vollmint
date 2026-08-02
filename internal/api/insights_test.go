package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetInsightsRequiresMonth(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/insights?view=household", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 for missing month", w.Code)
	}
}

func TestGetInsightsEmptyIsValid(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/insights?view=household&month=2026-07", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Insights []any `json:"insights"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Insights == nil {
		t.Fatal(`insights must be [], not null`)
	}
}

func TestGetInsights(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	// Netflix: price increase 15.49 -> 17.99 (+16%, +$2.50), monthly cadence.
	seedForecastTxn(t, s, "acct-api-ins", "scott", "nx-apr", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), "-15.49", "NETFLIX")
	seedForecastTxn(t, s, "acct-api-ins", "scott", "nx-may", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), "-15.49", "NETFLIX")
	seedForecastTxn(t, s, "acct-api-ins", "scott", "nx-jun", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX")

	req := httptest.NewRequest(http.MethodGet, "/api/insights?view=household&month=2026-07", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Insights []struct {
			Type   string `json:"type"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			Amount string `json:"amount"`
		} `json:"insights"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var found bool
	for _, in := range resp.Insights {
		if in.Type != "price_increase" {
			continue
		}
		found = true
		if in.Title != "Netflix price went up" {
			t.Fatalf("title = %q, want %q", in.Title, "Netflix price went up")
		}
		if in.Amount != "2.50" {
			t.Fatalf("amount = %q, want %q", in.Amount, "2.50")
		}
		if !strings.Contains(in.Body, "$15.49") || !strings.Contains(in.Body, "$17.99") {
			t.Fatalf("body missing expected amounts: %q", in.Body)
		}
	}
	if !found {
		t.Fatalf("missing price_increase insight: %+v", resp.Insights)
	}
}
