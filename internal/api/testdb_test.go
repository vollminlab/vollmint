package api

import (
	"context"
	"os"
	"testing"

	"github.com/vollminlab/vollmint/internal/migrate"
	"github.com/vollminlab/vollmint/internal/store"
	"github.com/vollminlab/vollmint/internal/testutil"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL not set (see plan Context section)")
	}
	testutil.SerializeDB(t, url)
	if err := migrate.Up(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(s.Close)
	for _, q := range []string{
		`TRUNCATE transactions, sync_runs, budgets, account_balance_snapshots RESTART IDENTITY CASCADE`,
		`DELETE FROM accounts WHERE id <> 'venmo'`,
		`DELETE FROM category_rules WHERE priority <> 1000`,
		`DELETE FROM categories WHERE name NOT IN (
			'Housing','Groceries','Dining','Transport','Utilities',
			'Subscriptions','Entertainment','Shopping','Health','Travel',
			'Vices','Paycheck','Savings','Transfer','Needs Venmo detail')`, // keep seed categories only
	} {
		if _, err := s.Pool.Exec(context.Background(), q); err != nil {
			t.Fatalf("reset (%s): %v", q, err)
		}
	}
	return s
}

// seedTxn inserts an account + one simplefin txn and returns its id.
func seedTxn(t *testing.T, s *store.Store, acct, owner, extID, posted, amount, desc string) int64 {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertAccounts(ctx, []store.Account{{ID: acct, Name: acct, Org: "t", Owner: owner}}); err != nil {
		t.Fatal(err)
	}
	p := mustDate(t, posted)
	if _, err := s.UpsertTransactions(ctx, []store.Txn{{
		Source: "simplefin", ExternalID: extID, AccountID: acct,
		Posted: p, Amount: amount, Description: desc, Payee: desc,
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
