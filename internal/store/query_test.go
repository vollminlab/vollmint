package store

import (
	"context"
	"testing"
	"time"
)

// seedTxn inserts one account (if new) and one categorized/uncategorized txn.
func seedTxn(t *testing.T, s *Store, acct, owner, extID, posted, amount, desc string, catID *int) int64 {
	t.Helper()
	ctx := context.Background()
	if err := s.UpsertAccounts(ctx, []Account{{ID: acct, Name: acct, Org: "t", Owner: owner}}); err != nil {
		t.Fatal(err)
	}
	p, err := time.Parse("2006-01-02", posted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertTransactions(ctx, []Txn{{
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
	if catID != nil {
		if _, err := s.Pool.Exec(ctx,
			`UPDATE transactions SET category_id=$1 WHERE id=$2`, *catID, id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestListTransactionsViewAndMonthFilter(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedTxn(t, s, "ally-s", "scott", "s1", "2026-07-05", "-10.00", "Coffee", nil)
	seedTxn(t, s, "ally-n", "nikki", "n1", "2026-07-06", "-20.00", "Books", nil)
	seedTxn(t, s, "ally-s", "scott", "s2", "2026-06-30", "-30.00", "OldMonth", nil)

	// household + month July → 2 rows (June excluded)
	got, err := s.ListTransactions(ctx, TxnFilter{View: "household", Month: "2026-07"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("household/July = %d rows, want 2", len(got))
	}
	// amounts are strings with 2 decimals
	if got[0].Amount == "" || got[0].Amount[0] != '-' {
		t.Fatalf("amount not a signed decimal string: %q", got[0].Amount)
	}

	// scott view → only scott-owned
	got, err = s.ListTransactions(ctx, TxnFilter{View: "scott", Month: "2026-07"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].EffectiveOwner != "scott" {
		t.Fatalf("scott/July = %v, want 1 scott row", got)
	}
}

func TestListTransactionsUncategorizedAndOwnerOverride(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedTxn(t, s, "joint1", "joint", "j1", "2026-07-10", "-40.00", "Dinner", nil)
	// owner_override moves a joint charge into scott's view
	if _, err := s.Pool.Exec(ctx,
		`UPDATE transactions SET owner_override='scott' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}

	// Get a real category ID for strengthening the uncategorized filter test
	var catID int
	if err := s.Pool.QueryRow(ctx, `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&catID); err != nil {
		t.Fatal(err)
	}

	// Seed a second transaction in the same scott/2026-07 window with a category
	id2 := seedTxn(t, s, "joint1", "joint", "j2", "2026-07-11", "-25.00", "Lunch", &catID)
	if _, err := s.Pool.Exec(ctx,
		`UPDATE transactions SET owner_override='scott' WHERE id=$1`, id2); err != nil {
		t.Fatal(err)
	}

	// Test with Uncategorized: true → should return only the first (uncategorized) row
	got, err := s.ListTransactions(ctx, TxnFilter{View: "scott", Month: "2026-07", Uncategorized: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].EffectiveOwner != "scott" || got[0].CategoryID != nil {
		t.Fatalf("uncategorized filter failed: got %d rows, want 1 uncategorized; %v", len(got), got)
	}

	// Test without Uncategorized filter → should return both rows
	got, err = s.ListTransactions(ctx, TxnFilter{View: "scott", Month: "2026-07"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("scott/July without filter = %d rows, want 2", len(got))
	}

	// joint view must NOT see either anymore (override wins)
	got, _ = s.ListTransactions(ctx, TxnFilter{View: "joint", Month: "2026-07"})
	if len(got) != 0 {
		t.Fatalf("joint view still sees overridden rows: %v", got)
	}
}
