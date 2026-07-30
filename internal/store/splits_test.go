package store

import (
	"context"
	"testing"
)

// seedSplitTxn creates an account + one posted, uncategorized transaction and
// returns its DB id. Amount is the parent amount (negative = spend). Thin
// wrapper around seedTxn (internal/store/query_test.go) to avoid duplicating
// the account-upsert + txn-upsert + id-lookup sequence.
func seedSplitTxn(t *testing.T, s *Store, extID, amount string) int64 {
	t.Helper()
	return seedTxn(t, s, "acct-split-test", "scott", extID, "2026-07-10", amount, "split test txn", nil)
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
