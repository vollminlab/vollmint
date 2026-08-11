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

func TestCaptureBalanceSnapshotsSkipsPartialNull(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	// balance set, balance_date NULL -> must be skipped
	if _, err := s.Pool.Exec(ctx, `
		INSERT INTO accounts (id, name, org, currency, owner, balance, balance_date)
		VALUES ('bal-only', 'Bal Only', 't', 'USD', 'scott', '50.00', NULL)`); err != nil {
		t.Fatal(err)
	}
	// balance NULL, balance_date set -> must be skipped
	if _, err := s.Pool.Exec(ctx, `
		INSERT INTO accounts (id, name, org, currency, owner, balance, balance_date)
		VALUES ('date-only', 'Date Only', 't', 'USD', 'scott', NULL, '2026-07-20')`); err != nil {
		t.Fatal(err)
	}

	if err := s.CaptureBalanceSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	if got := snapCount(t, s, "bal-only"); got != 0 {
		t.Errorf("bal-only (NULL date) snapshots = %d, want 0", got)
	}
	if got := snapCount(t, s, "date-only"); got != 0 {
		t.Errorf("date-only (NULL balance) snapshots = %d, want 0", got)
	}
}

func TestCreateManualAccount(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	id, err := s.CreateManualAccount(ctx, "My 401k!", "scott", "412000.00")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != "manual-my-401k" {
		t.Fatalf("id = %s, want manual-my-401k", id)
	}

	var org, currency, owner, balance string
	var isManual, active bool
	var dateIsToday bool
	if err := s.Pool.QueryRow(ctx, `
		SELECT org, currency, owner, balance::text, is_manual, active,
		       balance_date = CURRENT_DATE
		FROM accounts WHERE id = $1`, id).
		Scan(&org, &currency, &owner, &balance, &isManual, &active, &dateIsToday); err != nil {
		t.Fatal(err)
	}
	if org != "Manual" || currency != "USD" || owner != "scott" || balance != "412000.00" ||
		!isManual || !active || !dateIsToday {
		t.Errorf("row = org=%s currency=%s owner=%s balance=%s manual=%v active=%v today=%v",
			org, currency, owner, balance, isManual, active, dateIsToday)
	}
	if got := snapCount(t, s, id); got != 1 {
		t.Errorf("first snapshot rows = %d, want 1", got)
	}

	// Same derived id -> conflict.
	if _, err := s.CreateManualAccount(ctx, "my 401K", "scott", "1.00"); err != ErrConflict {
		t.Errorf("duplicate err = %v, want ErrConflict", err)
	}
	// Negative balance is a liability — allowed.
	if _, err := s.CreateManualAccount(ctx, "Mortgage", "joint", "-250000.00"); err != nil {
		t.Errorf("negative balance: %v", err)
	}
	// Garbage balance rejected before any SQL.
	if _, err := s.CreateManualAccount(ctx, "Bad", "scott", "12.345"); err != ErrInvalidAmount {
		t.Errorf("bad balance err = %v, want ErrInvalidAmount", err)
	}
}

func TestUpdateManualBalance(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	id, err := s.CreateManualAccount(ctx, "401k", "scott", "412000.00")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateManualBalance(ctx, id, "415250.00"); err != nil {
		t.Fatalf("update: %v", err)
	}
	var balance string
	var dateIsToday bool
	if err := s.Pool.QueryRow(ctx,
		`SELECT balance::text, balance_date = CURRENT_DATE FROM accounts WHERE id = $1`, id).
		Scan(&balance, &dateIsToday); err != nil {
		t.Fatal(err)
	}
	if balance != "415250.00" || !dateIsToday {
		t.Errorf("balance = %s today=%v, want 415250.00 true", balance, dateIsToday)
	}
	// Create-day snapshot was upserted, not duplicated: still one row, new value.
	if got := snapCount(t, s, id); got != 1 {
		t.Errorf("snapshot rows = %d, want 1", got)
	}

	// Synced accounts are owned by the sync — reject edits.
	if err := s.UpsertAccounts(ctx, []Account{
		{ID: "synced-1", Name: "S", Org: "t", Owner: "scott", Balance: "10.00", BalanceDate: day("2026-07-20")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateManualBalance(ctx, "synced-1", "99.00"); err != ErrNotManual {
		t.Errorf("synced err = %v, want ErrNotManual", err)
	}
	if err := s.UpdateManualBalance(ctx, "nope", "1.00"); err != ErrNotFound {
		t.Errorf("unknown err = %v, want ErrNotFound", err)
	}
	if err := s.UpdateManualBalance(ctx, id, "abc"); err != ErrInvalidAmount {
		t.Errorf("bad balance err = %v, want ErrInvalidAmount", err)
	}
}
