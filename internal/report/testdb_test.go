package report

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/vollminlab/vollmint/internal/migrate"
	"github.com/vollminlab/vollmint/internal/store"
	"github.com/vollminlab/vollmint/internal/testutil"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL not set")
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
		`TRUNCATE transactions, sync_runs, budgets RESTART IDENTITY CASCADE`,
		`DELETE FROM accounts WHERE id <> 'venmo'`,
		`DELETE FROM category_rules WHERE priority <> 1000`,
	} {
		if _, err := s.Pool.Exec(context.Background(), q); err != nil {
			t.Fatalf("reset (%s): %v", q, err)
		}
	}
	return s
}

// seedSpend inserts an account + a txn categorized as catName (or uncategorized
// if catName==""). amount is a signed decimal string.
func seedSpend(t *testing.T, s *store.Store, acct, owner, extID, posted, amount, catName string) {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertAccounts(ctx, []store.Account{{ID: acct, Name: acct, Org: "t", Owner: owner}}); err != nil {
		t.Fatal(err)
	}
	p, err := time.Parse("2006-01-02", posted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertTransactions(ctx, []store.Txn{{
		Source: "simplefin", ExternalID: extID, AccountID: acct,
		Posted: p, Amount: amount, Description: extID, Payee: extID,
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

func setBudget(t *testing.T, s *store.Store, catName, month, amount string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		INSERT INTO budgets (category_id, month, amount)
		VALUES ((SELECT id FROM categories WHERE name=$1), $2::date, $3)`,
		catName, month+"-01", amount); err != nil {
		t.Fatal(err)
	}
}
