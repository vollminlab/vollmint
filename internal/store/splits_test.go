package store

import (
	"context"
	"testing"
	"time"
)

// seedSplitTxn creates an account + one posted, categorized-later transaction
// and returns its DB id. Amount is the parent amount (negative = spend).
func seedSplitTxn(t *testing.T, s *Store, extID, amount string) int64 {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertAccounts(ctx, []Account{{
		ID: "acct-split-test", Name: "Split Test", Org: "test", Owner: "scott",
	}}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := s.UpsertTransactions(ctx, []Txn{{
		Source: "simplefin", ExternalID: extID, AccountID: "acct-split-test",
		Posted: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
		Amount: amount, Description: "split test txn", Payee: "SPLIT TEST",
	}}); err != nil {
		t.Fatalf("seed txn: %v", err)
	}
	var id int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT id FROM transactions WHERE source = 'simplefin' AND external_id = $1`,
		extID).Scan(&id); err != nil {
		t.Fatalf("lookup txn id: %v", err)
	}
	return id
}

// catID resolves a seed category name to its id.
func catID(t *testing.T, s *Store, name string) int {
	t.Helper()
	var id int
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM categories WHERE name = $1`, name).Scan(&id); err != nil {
		t.Fatalf("category %q: %v", name, err)
	}
	return id
}

func TestSplitsCascadeDeleteWithTransaction(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "cascade-1", "-50.00")

	_, err := s.Pool.Exec(ctx,
		`INSERT INTO transaction_splits (transaction_id, category_id, amount)
		 VALUES ($1, $2, '-30.00'), ($1, $3, '-20.00')`,
		id, catID(t, s, "Dining"), catID(t, s, "Groceries"))
	if err != nil {
		t.Fatalf("insert splits: %v", err)
	}

	if _, err := s.Pool.Exec(ctx, `DELETE FROM transactions WHERE id = $1`, id); err != nil {
		t.Fatalf("delete txn: %v", err)
	}
	var n int
	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM transaction_splits WHERE transaction_id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count splits: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 splits after parent delete, got %d", n)
	}
}
