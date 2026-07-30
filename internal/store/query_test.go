package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestListTransactionsQueryFilter(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedTxn(t, s, "ally-s", "scott", "s1", "2026-07-05", "-10.00", "Trader Joes", nil)
	seedTxn(t, s, "ally-s", "scott", "s2", "2026-07-06", "-20.00", "trader joes online", nil)
	seedTxn(t, s, "ally-s", "scott", "s3", "2026-07-07", "-30.00", "Coffee Shop", nil)

	got, err := s.ListTransactions(ctx, TxnFilter{View: "household", Month: "2026-07", Query: "trader"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("query filter = %d rows, want 2: %+v", len(got), got)
	}
	for _, r := range got {
		if r.Payee == "Coffee Shop" {
			t.Fatalf("query filter matched non-matching row: %+v", r)
		}
	}
}

func TestUpdateTransaction(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedTxn(t, s, "joint1", "joint", "j1", "2026-07-10", "-40.00", "Dinner", nil)

	var diningID int
	if err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID); err != nil {
		t.Fatal(err)
	}
	owner := "scott"
	if err := s.UpdateTransaction(ctx, id, TxnPatch{CategoryID: &diningID, OwnerOverride: &owner}); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListTransactions(ctx, TxnFilter{View: "household", Month: "2026-07"})
	if len(rows) != 1 || rows[0].CategoryName == nil || *rows[0].CategoryName != "Dining" {
		t.Fatalf("category not updated: %+v", rows)
	}
	if rows[0].EffectiveOwner != "scott" {
		t.Fatalf("owner override not applied: %+v", rows[0])
	}
}

func TestUpdateTransactionClearOwnerOverride(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedTxn(t, s, "joint1", "joint", "j1", "2026-07-10", "-40.00", "Dinner", nil)
	scott := "scott"
	if err := s.UpdateTransaction(ctx, id, TxnPatch{OwnerOverride: &scott}); err != nil {
		t.Fatal(err)
	}
	// empty-string sentinel clears the override back to NULL
	empty := ""
	if err := s.UpdateTransaction(ctx, id, TxnPatch{OwnerOverride: &empty}); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListTransactions(ctx, TxnFilter{View: "joint", Month: "2026-07"})
	if len(rows) != 1 {
		t.Fatalf("cleared override should return to joint view: %+v", rows)
	}
}

func TestUpdateTransactionNotFound(t *testing.T) {
	s := testDB(t)
	diningID := 1
	err := s.UpdateTransaction(context.Background(), 999999, TxnPatch{CategoryID: &diningID})
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCategoryCRUD(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	// list includes seeds
	cats, err := s.ListCategories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) == 0 {
		t.Fatal("expected seed categories")
	}
	// create
	id, err := s.CreateCategory(ctx, "Pets", "spend", false)
	if err != nil {
		t.Fatal(err)
	}
	// update
	if err := s.UpdateCategory(ctx, id, CategoryPatch{IsVice: boolp(true)}); err != nil {
		t.Fatal(err)
	}
	cats, _ = s.ListCategories(ctx)
	var found bool
	for _, c := range cats {
		if c.ID == id && c.Name == "Pets" && c.IsVice {
			found = true
		}
	}
	if !found {
		t.Fatal("updated Pets category not found or is_vice not set")
	}
}

func boolp(b bool) *bool { return &b }

func TestRuleCRUD(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	var diningID int
	_ = s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Dining'`).Scan(&diningID)

	id, err := s.CreateRule(ctx, 10, "substring", "CHIPOTLE", diningID)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.ListRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range rules {
		if r.ID == id && r.Pattern == "CHIPOTLE" && r.CategoryID == diningID {
			found = true
		}
	}
	if !found {
		t.Fatal("created rule not listed")
	}
	if err := s.DeleteRule(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRule(ctx, id); err != ErrNotFound {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
}

func TestCreateRuleBadCategory(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	_, err := s.CreateRule(ctx, 1, "substring", "X", 999999)
	if err == nil {
		t.Fatal("want error for invalid category_id")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want PgError, got %T", err)
	}
	if pgErr.Code != "23503" {
		t.Fatalf("want FK error code 23503, got %s", pgErr.Code)
	}
}

func TestBudgetGetPut(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	var groceriesID int
	err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name = 'Groceries'`).Scan(&groceriesID)
	if err != nil {
		t.Fatalf("get groceries id: %v", err)
	}

	if err := s.PutBudgets(ctx, "2026-07", []BudgetItem{{CategoryID: groceriesID, Amount: "120.00"}}); err != nil {
		t.Fatalf("put budgets: %v", err)
	}
	items, err := s.GetBudgets(ctx, "2026-07")
	if err != nil {
		t.Fatalf("get budgets: %v", err)
	}
	if len(items) != 1 || items[0].Amount != "120.00" {
		t.Fatalf("unexpected budgets: %+v", items)
	}

	// whole-month replace: nil clears the month
	if err := s.PutBudgets(ctx, "2026-07", nil); err != nil {
		t.Fatalf("clear budgets: %v", err)
	}
	items, err = s.GetBudgets(ctx, "2026-07")
	if err != nil {
		t.Fatalf("get budgets after clear: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 budgets after clear, got %d", len(items))
	}
}

func TestPutBudgetsBadAmount(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	var groceriesID int
	err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name = 'Groceries'`).Scan(&groceriesID)
	if err != nil {
		t.Fatalf("get groceries id: %v", err)
	}

	// bad amount should return ErrInvalidAmount
	err = s.PutBudgets(ctx, "2026-07", []BudgetItem{{CategoryID: groceriesID, Amount: "12.3.4"}})
	if err == nil {
		t.Fatal("want error for invalid amount")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
}

func TestListTransactionsEmbedsSplits(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "embed-1", "-50.00")
	if err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	rows, err := s.ListTransactions(ctx, TxnFilter{View: "household", Month: "2026-07"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found *TxnRow
	for i := range rows {
		if rows[i].ID == id {
			found = &rows[i]
		} else if rows[i].Splits == nil {
			t.Fatalf("unsplit txn %d has nil Splits — want empty slice", rows[i].ID)
		}
	}
	if found == nil {
		t.Fatal("split txn not in listing")
	}
	if len(found.Splits) != 2 || found.Splits[0].Category != "Dining" {
		t.Fatalf("splits not embedded: %+v", found.Splits)
	}
}

func TestListTransactionsCategoryFilterIsSplitAware(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "filter-1", "-50.00")
	dining, groceries := catID(t, s, "Dining"), catID(t, s, "Groceries")

	// Parent categorized Dining, then split into Dining + Groceries.
	if err := s.UpdateTransaction(ctx, id, TxnPatch{CategoryID: &dining}); err != nil {
		t.Fatalf("categorize: %v", err)
	}
	if err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: dining, Amount: "-30.00"},
		{CategoryID: groceries, Amount: "-20.00"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	// Filter by a split part's category → parent row returned.
	rows, err := s.ListTransactions(ctx, TxnFilter{View: "household", Month: "2026-07", CategoryID: &groceries})
	if err != nil {
		t.Fatalf("list groceries: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != id {
		t.Fatalf("split-part filter should match parent, got %+v", rows)
	}
}

func TestListTransactionsUnsplitStillMatchesOwnCategory(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "unsplit-1", "-25.00")
	dining := catID(t, s, "Dining")
	if err := s.UpdateTransaction(ctx, id, TxnPatch{CategoryID: &dining}); err != nil {
		t.Fatalf("categorize: %v", err)
	}
	rows, err := s.ListTransactions(ctx, TxnFilter{View: "household", Month: "2026-07", CategoryID: &dining})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != id {
		t.Fatalf("unsplit txn should match its own category, got %+v", rows)
	}
}

func TestGetTransaction(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	id := seedSplitTxn(t, s, "get-1", "-50.00")
	if err := s.ReplaceSplits(ctx, id, []SplitInput{
		{CategoryID: catID(t, s, "Dining"), Amount: "-30.00"},
		{CategoryID: catID(t, s, "Groceries"), Amount: "-20.00"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	txn, err := s.GetTransaction(ctx, id)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if txn.ID != id || txn.Amount != "-50.00" || len(txn.Splits) != 2 {
		t.Fatalf("wrong txn: %+v", txn)
	}

	if _, err := s.GetTransaction(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing id, got %v", err)
	}
}

func TestSyncStatus(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sync_runs (kind, status, rows_upserted, detail, finished)
		VALUES ('simplefin','ok',5,'',now()), ('venmo_csv','failed',0,'bad header',now())`)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := s.SyncStatus(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(runs))
	}
	if runs[0].Kind == "" || runs[0].Status == "" {
		t.Fatalf("run not populated: %+v", runs[0])
	}
}
