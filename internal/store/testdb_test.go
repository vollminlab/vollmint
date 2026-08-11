package store

import (
	"context"
	"os"
	"testing"

	"github.com/vollminlab/vollmint/internal/migrate"
	"github.com/vollminlab/vollmint/internal/testutil"
)

// testDB returns a Store on TEST_DATABASE_URL with migrations applied and
// mutable tables truncated (seed rows in categories/accounts/rules survive
// via re-migration after TRUNCATE ... CASCADE would be complex; instead we
// truncate only transaction-ish tables and restore the venmo account).
func testDB(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL not set (see README dev section)")
	}
	testutil.SerializeDB(t, url)
	if err := migrate.Up(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s, err := New(context.Background(), url)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(s.Close)
	for _, q := range []string{
		`TRUNCATE transactions, sync_runs, budgets, account_balance_snapshots RESTART IDENTITY CASCADE`,
		`DELETE FROM accounts WHERE id <> 'venmo'`,
		`DELETE FROM category_rules WHERE priority <> 1000`, // keep only the seed VENMO rule
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
