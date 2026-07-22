package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/vollminlab/vollmint/internal/store"
)

func day(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

func seedTxn(t *testing.T, s *store.Store, extID, desc, amount string) int64 {
	t.Helper()
	_, err := s.UpsertTransactions(context.Background(), []store.Txn{{
		Source: "simplefin", ExternalID: extID, AccountID: "venmo",
		Posted: day("2026-07-10"), Amount: amount, Description: desc, Payee: desc,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM transactions WHERE source='simplefin' AND external_id=$1`, extID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestApplyRulesCategorizesUncategorizedOnly(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	var dining, groceries int
	s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Dining'`).Scan(&dining)
	s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Groceries'`).Scan(&groceries)
	// Lower priority number wins; the seed VENMO rule sits at 1000.
	s.Pool.Exec(ctx, `INSERT INTO category_rules (priority, match_type, pattern, category_id) VALUES
		(10, 'substring', 'chipotle', $1), (20, 'regex', '(?i)^wegmans', $2)`, dining, groceries)

	id1 := seedTxn(t, s, "r1", "CHIPOTLE 2291", "-14.62")
	id2 := seedTxn(t, s, "r2", "WEGMANS #44", "-88.10")
	id3 := seedTxn(t, s, "r3", "VENMO PAYMENT 55", "-32.00")
	id4 := seedTxn(t, s, "r4", "MYSTERY VENDOR", "-5.00")

	// Pre-categorized rows must not be overwritten.
	s.Pool.Exec(ctx, `UPDATE transactions SET category_id=$1 WHERE id=$2`, groceries, id1)

	n, err := ApplyRules(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 { // r2 (regex) + r3 (seed VENMO rule); r1 already set, r4 no match
		t.Fatalf("want 2 categorized, got %d", n)
	}
	check := func(id int64, want string) {
		var got string
		s.Pool.QueryRow(ctx, `SELECT coalesce(c.name,'') FROM transactions t
			LEFT JOIN categories c ON c.id=t.category_id WHERE t.id=$1`, id).Scan(&got)
		if got != want {
			t.Errorf("txn %d: category %q, want %q", id, got, want)
		}
	}
	check(id1, "Groceries")          // untouched
	check(id2, "Groceries")          // regex rule
	check(id3, "Needs Venmo detail") // seed VENMO rule
	check(id4, "")                   // uncategorized queue
}
