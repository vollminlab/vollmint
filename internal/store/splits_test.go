package store

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestReplaceSplitsHappyPath(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "happy-1", "-50.00")
	dining, groceries := catID(t, s, "Dining"), catID(t, s, "Groceries")

	err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: dining, Amount: "-30.00", Note: "dinner"},
		{CategoryID: groceries, Amount: "-20.00"},
	})
	if err != nil {
		t.Fatalf("ReplaceSplits: %v", err)
	}

	m, err := s.SplitsByTxnIDs(ctx, []int64{id})
	if err != nil {
		t.Fatalf("SplitsByTxnIDs: %v", err)
	}
	sp := m[id]
	if len(sp) != 2 {
		t.Fatalf("want 2 splits, got %d", len(sp))
	}
	if sp[0].Category != "Dining" || sp[0].Amount != "-30.00" || sp[0].Note != "dinner" {
		t.Fatalf("first split wrong: %+v", sp[0])
	}
	if sp[1].Category != "Groceries" || sp[1].Amount != "-20.00" {
		t.Fatalf("second split wrong: %+v", sp[1])
	}
}

func TestReplaceSplitsTooFewParts(t *testing.T) {
	s := testDB(t)
	id := seedSplitTxn(t, s, "few-1", "-50.00")
	err := s.ReplaceSplits(context.Background(), id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-50.00"},
	})
	if !errors.Is(err, ErrSplitTooFew) {
		t.Fatalf("want ErrSplitTooFew, got %v", err)
	}
}

func TestReplaceSplitsBadAmount(t *testing.T) {
	s := testDB(t)
	id := seedSplitTxn(t, s, "bad-amt-1", "-50.00")
	err := s.ReplaceSplits(context.Background(), id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "thirty"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	})
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
}

func TestReplaceSplitsSumMismatch(t *testing.T) {
	s := testDB(t)
	id := seedSplitTxn(t, s, "sum-1", "-50.00")
	err := s.ReplaceSplits(context.Background(), id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-15.00"},
	})
	var sumErr *SplitSumError
	if !errors.As(err, &sumErr) {
		t.Fatalf("want *SplitSumError, got %v", err)
	}
	if sumErr.Expected != "-50.00" || sumErr.Received != "-45.00" {
		t.Fatalf("wrong totals in error: %+v", sumErr)
	}
}

func TestReplaceSplitsSignMismatch(t *testing.T) {
	s := testDB(t)
	id := seedSplitTxn(t, s, "sign-1", "-50.00")
	// -70 + +20 sums to -50 exactly — sign check must fire before sum check.
	err := s.ReplaceSplits(context.Background(), id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-70.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "20.00"},
	})
	if !errors.Is(err, ErrSplitSign) {
		t.Fatalf("want ErrSplitSign, got %v", err)
	}
}

func TestReplaceSplitsNotFound(t *testing.T) {
	s := testDB(t)
	err := s.ReplaceSplits(context.Background(), 999999, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestReplaceSplitsRejectsPendingParent(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "pend-1", "-50.00")
	if _, err := s.Pool.Exec(ctx, `UPDATE transactions SET pending = true WHERE id = $1`, id); err != nil {
		t.Fatalf("mark pending: %v", err)
	}
	err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	})
	if !errors.Is(err, ErrSplitParent) {
		t.Fatalf("want ErrSplitParent, got %v", err)
	}
}

func TestReplaceSplitsRejectsTransferParent(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "xfer-1", "-50.00")
	if _, err := s.Pool.Exec(ctx,
		`UPDATE transactions SET transfer_peer_id = id WHERE id = $1`, id); err != nil {
		t.Fatalf("mark transfer: %v", err)
	}
	err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	})
	if !errors.Is(err, ErrSplitParent) {
		t.Fatalf("want ErrSplitParent, got %v", err)
	}
}

func TestReplaceSplitsUnknownCategory(t *testing.T) {
	s := testDB(t)
	id := seedSplitTxn(t, s, "cat-1", "-50.00")
	err := s.ReplaceSplits(context.Background(), id, []SplitInput{
		{CategoryID: 999999, Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("want FK violation 23503, got %v", err)
	}
}

func TestReplaceSplitsIsAtomicReplace(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "atomic-1", "-50.00")
	dining, groceries, ent := catID(t, s, "Dining"), catID(t, s, "Groceries"), catID(t, s, "Entertainment")

	if err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: dining, Amount: "-30.00"},
		{CategoryID: groceries, Amount: "-20.00"},
	}); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	if err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: ent, Amount: "-45.00"},
		{CategoryID: groceries, Amount: "-5.00"},
	}); err != nil {
		t.Fatalf("second replace: %v", err)
	}
	m, err := s.SplitsByTxnIDs(ctx, []int64{id})
	if err != nil {
		t.Fatalf("SplitsByTxnIDs: %v", err)
	}
	sp := m[id]
	if len(sp) != 2 || sp[0].Category != "Entertainment" || sp[1].Amount != "-5.00" {
		t.Fatalf("second set did not fully replace first: %+v", sp)
	}
}

func TestDeleteSplitsIdempotent(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "del-1", "-50.00")
	if err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := s.DeleteSplits(ctx, id); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	if err := s.DeleteSplits(ctx, id); err != nil {
		t.Fatalf("second delete should be a no-op: %v", err)
	}
	m, _ := s.SplitsByTxnIDs(ctx, []int64{id})
	if len(m[id]) != 0 {
		t.Fatalf("splits remain after delete: %+v", m[id])
	}
}

func TestSplitsByTxnIDsEmptyInput(t *testing.T) {
	s := testDB(t)
	m, err := s.SplitsByTxnIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("SplitsByTxnIDs(nil): %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("want empty map, got %+v", m)
	}
}
