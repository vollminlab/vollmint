package store

import (
	"context"
	"testing"
)

func snapCount(t *testing.T, s *Store, id string) int {
	t.Helper()
	var n int
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM account_balance_snapshots WHERE account_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func snapBalance(t *testing.T, s *Store, id, date string) string {
	t.Helper()
	var bal string
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT balance::text FROM account_balance_snapshots WHERE account_id = $1 AND snapshot_date = $2::date`,
		id, date).Scan(&bal); err != nil {
		t.Fatal(err)
	}
	return bal
}

func TestCaptureBalanceSnapshots(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	// a1 has a balance+date; a2 has neither (like Venmo) and must be skipped.
	if err := s.UpsertAccounts(ctx, []Account{
		{ID: "a1", Name: "A1", Org: "t", Owner: "scott", Balance: "100.00", BalanceDate: day("2026-07-20")},
		{ID: "a2", Name: "A2", Org: "t", Owner: "scott"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.CaptureBalanceSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	if got := snapCount(t, s, "a1"); got != 1 {
		t.Fatalf("a1 snapshots = %d, want 1", got)
	}
	if got := snapCount(t, s, "a2"); got != 0 {
		t.Fatalf("a2 (no balance) snapshots = %d, want 0", got)
	}
	if got := snapBalance(t, s, "a1", "2026-07-20"); got != "100.00" {
		t.Fatalf("a1 balance = %s, want 100.00", got)
	}

	// Same balance_date, new balance (evening sync) — overwrites, no new row.
	if err := s.UpsertAccounts(ctx, []Account{
		{ID: "a1", Name: "A1", Org: "t", Owner: "scott", Balance: "150.00", BalanceDate: day("2026-07-20")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CaptureBalanceSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	if got := snapCount(t, s, "a1"); got != 1 {
		t.Fatalf("after same-date recapture: snapshots = %d, want 1", got)
	}
	if got := snapBalance(t, s, "a1", "2026-07-20"); got != "150.00" {
		t.Fatalf("after same-date recapture: balance = %s, want 150.00", got)
	}

	// New balance_date — second row appears, first row untouched.
	if err := s.UpsertAccounts(ctx, []Account{
		{ID: "a1", Name: "A1", Org: "t", Owner: "scott", Balance: "175.00", BalanceDate: day("2026-07-21")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.CaptureBalanceSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	if got := snapCount(t, s, "a1"); got != 2 {
		t.Fatalf("after new-date capture: snapshots = %d, want 2", got)
	}
	if got := snapBalance(t, s, "a1", "2026-07-21"); got != "175.00" {
		t.Fatalf("new-date balance = %s, want 175.00", got)
	}
}
