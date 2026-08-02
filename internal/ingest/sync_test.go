package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vollminlab/vollmint/internal/simplefin"
	"github.com/vollminlab/vollmint/internal/store"
)

func fakeBridge(t *testing.T, body string) *simplefin.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	c := simplefin.New("https://u:p@" + srv.Listener.Addr().String())
	simplefin.ForceHTTP(c) // test hook, added in Step 3
	return c
}

func TestSyncEndToEnd(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	posted := time.Now().AddDate(0, 0, -2).Unix()
	c := fakeBridge(t, fmt.Sprintf(`{"errors":[],"accounts":[{
		"id":"ally-1","name":"Ally Checking","currency":"USD",
		"balance":"900.00","balance-date":%d,
		"org":{"name":"Ally Bank","domain":"ally.com"},
		"transactions":[
		 {"id":"s1","posted":%d,"amount":"-14.62","description":"CHIPOTLE 2291","pending":false},
		 {"id":"s2","posted":%d,"amount":"-32.00","description":"VENMO PAYMENT 4111","pending":false}
		]}]}`, posted, posted, posted))

	res, err := Sync(ctx, s, c, "scott")
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 2 {
		t.Fatalf("upserted=%d want 2", res.Upserted)
	}

	// sync_runs row recorded as ok
	var status string
	var rowsUp int
	if err := s.Pool.QueryRow(ctx,
		`SELECT status, rows_upserted FROM sync_runs ORDER BY id DESC LIMIT 1`).Scan(&status, &rowsUp); err != nil {
		t.Fatal(err)
	}
	if status != "ok" || rowsUp != 2 {
		t.Fatalf("sync_runs: status=%q rows=%d", status, rowsUp)
	}

	// rules ran: VENMO txn landed in the needs-detail bucket
	var cat string
	s.Pool.QueryRow(ctx, `SELECT coalesce(c.name,'') FROM transactions t
		LEFT JOIN categories c ON c.id=t.category_id
		WHERE t.external_id='s2'`).Scan(&cat)
	if cat != "Needs Venmo detail" {
		t.Fatalf("VENMO txn category=%q", cat)
	}

	// new account defaulted to the fallback owner
	var owner string
	s.Pool.QueryRow(ctx, `SELECT owner FROM accounts WHERE id='ally-1'`).Scan(&owner)
	if owner != "scott" {
		t.Fatalf("owner=%q", owner)
	}
}

func TestSyncRecordsFailure(t *testing.T) {
	s := testDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Payment Required", http.StatusPaymentRequired)
	}))
	defer srv.Close()
	c := simplefin.New("https://u:p@" + srv.Listener.Addr().String())
	simplefin.ForceHTTP(c)

	if _, err := Sync(context.Background(), s, c, "scott"); err == nil {
		t.Fatal("want error on 402")
	}
	var status, detail string
	s.Pool.QueryRow(context.Background(),
		`SELECT status, detail FROM sync_runs ORDER BY id DESC LIMIT 1`).Scan(&status, &detail)
	if status != "failed" || detail == "" {
		t.Fatalf("failure not recorded: status=%q detail=%q", status, detail)
	}
}

func TestSweepStalePending(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedAccount(t, s, "ally", "scott")
	s.Pool.Exec(ctx, `INSERT INTO transactions (source, external_id, account_id, posted, amount, pending, updated_at)
		VALUES ('simplefin','old-pend','ally', current_date - 30, '-5.00', true, now() - interval '20 days'),
		       ('simplefin','new-pend','ally', current_date - 1,  '-6.00', true, now())`)
	n, err := SweepStalePending(ctx, s, 14)
	if err != nil || n != 1 {
		t.Fatalf("swept=%d err=%v (want 1)", n, err)
	}
	var count int
	s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE pending`).Scan(&count)
	if count != 1 {
		t.Fatalf("remaining pending=%d want 1", count)
	}
}

func TestWindowStartOverlap(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	started := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if _, err := s.Pool.Exec(ctx, `INSERT INTO sync_runs (kind, status, started)
		VALUES ('simplefin','ok',$1), ('simplefin','failed',now())`, started); err != nil {
		t.Fatal(err)
	}
	got, err := windowStart(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	want := started.AddDate(0, 0, -7)
	if !got.Equal(want) {
		t.Fatalf("windowStart=%v want %v", got, want)
	}
}

func TestCleanStaleSplits(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	if err := s.UpsertAccounts(ctx, []store.Account{{
		ID: "acct-stale", Name: "Stale Test", Org: "test", Owner: "scott",
	}}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := s.UpsertTransactions(ctx, []store.Txn{{
		Source: "simplefin", ExternalID: "stale-1", AccountID: "acct-stale",
		Posted: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC),
		Amount: "-50.00", Description: "stale split test", Payee: "STALE TEST",
	}}); err != nil {
		t.Fatalf("seed txn: %v", err)
	}
	var id int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT id FROM transactions WHERE source='simplefin' AND external_id='stale-1'`).Scan(&id); err != nil {
		t.Fatalf("lookup id: %v", err)
	}
	var dining, groceries int
	if err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Dining'`).Scan(&dining); err != nil {
		t.Fatalf("dining id: %v", err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT id FROM categories WHERE name='Groceries'`).Scan(&groceries); err != nil {
		t.Fatalf("groceries id: %v", err)
	}
	if err := s.ReplaceSplits(ctx, id, []store.SplitInput{
		{CategoryID: dining, Amount: "-30.00"},
		{CategoryID: groceries, Amount: "-20.00"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}

	// Consistent splits survive cleanup.
	n, err := CleanStaleSplits(ctx, s)
	if err != nil {
		t.Fatalf("clean (consistent): %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 deleted while consistent, got %d", n)
	}

	// Simulate a sync re-upsert changing the parent amount → splits are stale.
	if _, err := s.Pool.Exec(ctx, `UPDATE transactions SET amount = '-60.00' WHERE id = $1`, id); err != nil {
		t.Fatalf("mutate amount: %v", err)
	}
	n, err = CleanStaleSplits(ctx, s)
	if err != nil {
		t.Fatalf("clean (stale): %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 stale split rows deleted, got %d", n)
	}
	var remain int
	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM transaction_splits WHERE transaction_id = $1`, id).Scan(&remain); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remain != 0 {
		t.Fatalf("stale splits remain: %d", remain)
	}
}
