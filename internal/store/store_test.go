package store

import (
	"context"
	"testing"
	"time"
)

func day(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

func TestUpsertTransactionsIsIdempotent(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	err := s.UpsertAccounts(ctx, []Account{{ID: "act-1", Name: "Ally Checking", Org: "Ally Bank", Owner: "scott", Balance: "1000.00", BalanceDate: day("2026-07-20")}})
	if err != nil {
		t.Fatalf("UpsertAccounts: %v", err)
	}

	txns := []Txn{
		{Source: "simplefin", ExternalID: "t1", AccountID: "act-1", Posted: day("2026-07-18"), Amount: "-14.62", Description: "CHIPOTLE 2291", Payee: "CHIPOTLE 2291", Raw: []byte(`{"id":"t1"}`)},
		{Source: "simplefin", ExternalID: "t2", AccountID: "act-1", Posted: day("2026-07-19"), Amount: "-41.87", Description: "DOORDASH", Payee: "DOORDASH", Raw: []byte(`{"id":"t2"}`)},
	}
	n, err := s.UpsertTransactions(ctx, txns)
	if err != nil || n != 2 {
		t.Fatalf("first upsert: n=%d err=%v", n, err)
	}

	// Re-upsert with one changed description — must update, never duplicate.
	txns[1].Description = "DOORDASH *LUIGIS"
	if _, err := s.UpsertTransactions(ctx, txns); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	var count int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("want 2 rows, got %d (err=%v)", count, err)
	}
	var desc string
	if err := s.Pool.QueryRow(ctx, `SELECT description FROM transactions WHERE external_id='t2'`).Scan(&desc); err != nil || desc != "DOORDASH *LUIGIS" {
		t.Fatalf("update not applied: %q err=%v", desc, err)
	}
}

func TestUpsertRejectsBadAmount(t *testing.T) {
	s := testDB(t)
	_, err := s.UpsertTransactions(context.Background(), []Txn{{Source: "simplefin", ExternalID: "x", AccountID: "venmo", Posted: day("2026-07-01"), Amount: "12.3.4"}})
	if err == nil {
		t.Fatal("expected error for malformed amount")
	}
}
