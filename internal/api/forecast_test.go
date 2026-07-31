package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vollminlab/vollmint/internal/store"
)

// seedForecastTxn seeds one simplefin transaction with an explicit payee
// and a time.Time posted date. Mirrors the api package's seedTxn helper
// (same treatment as report.seedBill), but accepts an explicit payee so
// the same payee can recur across months for forecast detection.
func seedForecastTxn(t *testing.T, s *store.Store, acct, owner, extID string, posted time.Time, amount, payee string) int64 {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertAccounts(ctx, []store.Account{{ID: acct, Name: acct, Org: "t", Owner: owner}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertTransactions(ctx, []store.Txn{{
		Source: "simplefin", ExternalID: extID, AccountID: acct,
		Posted: posted, Amount: amount, Description: extID, Payee: payee,
	}}); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT id FROM transactions WHERE source='simplefin' AND external_id=$1`, extID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGetForecast(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	// Three months of the same payee → forecastable.
	for i, d := range []time.Time{
		time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
	} {
		seedForecastTxn(t, s, "acct-fc-api", "scott", "vz-api-"+itoa(i), d, "-120.00", "VERIZON WIRELESS")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/forecast?view=household&month=2026-07", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Forecast struct {
			Month string `json:"month"`
			Bills []struct {
				Payee          string `json:"payee"`
				PredictedDay   int    `json:"predicted_day"`
				ExpectedAmount string `json:"expected_amount"`
				Paid           bool   `json:"paid"`
			} `json:"bills"`
			RemainingExpected string `json:"remaining_expected"`
		} `json:"forecast"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Forecast.Month != "2026-07" || len(resp.Forecast.Bills) != 1 {
		t.Fatalf("bad envelope: %s", w.Body.String())
	}
	b := resp.Forecast.Bills[0]
	if b.Payee != "VERIZON WIRELESS" {
		t.Fatalf("payee %q, want VERIZON WIRELESS", b.Payee)
	}
	if b.ExpectedAmount != "120.00" {
		t.Fatalf("expected amount %q, want 120.00", b.ExpectedAmount)
	}
	if b.PredictedDay != 14 {
		t.Fatalf("predicted day %d, want 14", b.PredictedDay)
	}
	if b.Paid {
		t.Fatal("bill should not be paid")
	}
	if resp.Forecast.RemainingExpected != "120.00" {
		t.Fatalf("remaining %q, want 120.00", resp.Forecast.RemainingExpected)
	}
}

func TestGetForecastRequiresMonth(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/forecast?view=household", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 for missing month", w.Code)
	}
}

func TestGetForecastEmptyIsValid(t *testing.T) {
	s := testStore(t)
	h := New(s).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/forecast?view=household&month=2026-07", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp struct {
		Forecast struct {
			Bills             []any  `json:"bills"`
			RemainingExpected string `json:"remaining_expected"`
		} `json:"forecast"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Forecast.Bills == nil {
		t.Fatal(`bills must be [], not null`)
	}
	if resp.Forecast.RemainingExpected != "0" {
		t.Fatalf("remaining %q, want 0", resp.Forecast.RemainingExpected)
	}
}
