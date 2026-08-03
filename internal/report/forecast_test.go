package report

import (
	"context"
	"testing"
	"time"

	"github.com/vollminlab/vollmint/internal/store"
)

// seedBill seeds one categorized transaction with an explicit payee.
// Mirrors seedSpend but payee is a parameter so the same payee can recur
// across months (forecast detection groups by payee).
func seedBill(t *testing.T, s *store.Store, acct, owner, extID string, posted time.Time, amount, payee, catName string) {
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
	if catName != "" {
		if _, err := s.Pool.Exec(ctx, `
			UPDATE transactions SET category_id=(SELECT id FROM categories WHERE name=$1)
			WHERE source='simplefin' AND external_id=$2`, catName, extID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestForecastDetectsMonthlyBill(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// VERIZON: charged on ~the 14th in Apr, May, Jun — 3 distinct months,
	// present in 2+ of the 3 months before July.
	seedBill(t, s, "acct-fc", "scott", "vz-apr", time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC), "-120.00", "VERIZON WIRELESS", "Utilities")
	seedBill(t, s, "acct-fc", "scott", "vz-may", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), "-121.00", "VERIZON WIRELESS", "Utilities")
	seedBill(t, s, "acct-fc", "scott", "vz-jun", time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), "-128.42", "VERIZON WIRELESS", "Utilities")

	f, err := Forecast(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(f.Bills) != 1 {
		t.Fatalf("want 1 bill, got %d: %+v", len(f.Bills), f.Bills)
	}
	b := f.Bills[0]
	if b.Payee != "VERIZON WIRELESS" || b.Category != "Utilities" {
		t.Fatalf("wrong bill: %+v", b)
	}
	if b.PredictedDay != 14 {
		t.Fatalf("predicted day %d, want 14 (median of 14,15,13)", b.PredictedDay)
	}
	if b.ExpectedAmount != "128.42" {
		t.Fatalf("expected amount %q, want latest charge 128.42", b.ExpectedAmount)
	}
	if b.Paid {
		t.Fatal("no July charge — should be unpaid")
	}
	if f.RemainingExpected != "128.42" {
		t.Fatalf("remaining %q, want 128.42", f.RemainingExpected)
	}
}

func TestForecastExcludesDeadAndP2P(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Dead: 3 months long ago, nothing in the 3 months before July.
	seedBill(t, s, "acct-fc", "scott", "gym-1", time.Date(2025, 10, 5, 0, 0, 0, 0, time.UTC), "-40.00", "OLD GYM", "Health")
	seedBill(t, s, "acct-fc", "scott", "gym-2", time.Date(2025, 11, 5, 0, 0, 0, 0, time.UTC), "-40.00", "OLD GYM", "Health")
	seedBill(t, s, "acct-fc", "scott", "gym-3", time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC), "-40.00", "OLD GYM", "Health")

	// P2P: monthly Venmo, would otherwise qualify.
	seedBill(t, s, "acct-fc", "scott", "vm-1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC), "-25.00", "Venmo Payment", "Dining")
	seedBill(t, s, "acct-fc", "scott", "vm-2", time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "-25.00", "Venmo Payment", "Dining")
	seedBill(t, s, "acct-fc", "scott", "vm-3", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC), "-25.00", "Venmo Payment", "Dining")

	f, err := Forecast(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(f.Bills) != 0 {
		t.Fatalf("want 0 bills (dead + P2P excluded), got %+v", f.Bills)
	}
	if f.RemainingExpected != "0.00" {
		t.Fatalf("remaining %q, want 0.00", f.RemainingExpected)
	}
}

func TestForecastSameDayTiebreak(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Qualifying cadence (Apr, May, Jun), but June has two charges on the
	// same calendar day — DISTINCT ON must deterministically pick the
	// later-inserted (higher id) row, not whichever the planner happens to
	// return first.
	seedBill(t, s, "acct-fc", "scott", "dc-apr", time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC), "-50.00", "DOUBLE CHARGE CO", "Utilities")
	seedBill(t, s, "acct-fc", "scott", "dc-may", time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC), "-50.00", "DOUBLE CHARGE CO", "Utilities")
	seedBill(t, s, "acct-fc", "scott", "dc-jun-a", time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), "-50.00", "DOUBLE CHARGE CO", "Utilities")
	seedBill(t, s, "acct-fc", "scott", "dc-jun-b", time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), "-75.00", "DOUBLE CHARGE CO", "Utilities")

	f, err := Forecast(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(f.Bills) != 1 {
		t.Fatalf("want 1 bill, got %d: %+v", len(f.Bills), f.Bills)
	}
	if f.Bills[0].ExpectedAmount != "75.00" {
		t.Fatalf("expected amount %q, want 75.00 (higher-id, later-inserted same-day charge)", f.Bills[0].ExpectedAmount)
	}
}

func TestForecastPaidMatching(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seedBill(t, s, "acct-fc", "scott", "nf-apr", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-fc", "scott", "nf-may", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-fc", "scott", "nf-jun", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX", "Subscriptions")
	// Paid this month on the 19th.
	seedBill(t, s, "acct-fc", "scott", "nf-jul", time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX", "Subscriptions")

	// A second, unpaid bill so ordering is observable.
	seedBill(t, s, "acct-fc", "scott", "sp-apr", time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC), "-11.99", "SPOTIFY", "Subscriptions")
	seedBill(t, s, "acct-fc", "scott", "sp-may", time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC), "-11.99", "SPOTIFY", "Subscriptions")
	seedBill(t, s, "acct-fc", "scott", "sp-jun", time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC), "-11.99", "SPOTIFY", "Subscriptions")

	f, err := Forecast(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatalf("Forecast: %v", err)
	}
	if len(f.Bills) != 2 {
		t.Fatalf("want 2 bills, got %+v", f.Bills)
	}
	// Unpaid first, paid sinks below.
	if f.Bills[0].Payee != "SPOTIFY" || f.Bills[0].Paid {
		t.Fatalf("first bill should be unpaid SPOTIFY: %+v", f.Bills[0])
	}
	nf := f.Bills[1]
	if !nf.Paid || nf.PaidDate != "2026-07-19" || nf.PaidAmount != "17.99" {
		t.Fatalf("NETFLIX paid fields wrong: %+v", nf)
	}
	if f.RemainingExpected != "11.99" {
		t.Fatalf("remaining %q, want 11.99 (only Spotify unpaid)", f.RemainingExpected)
	}
}
