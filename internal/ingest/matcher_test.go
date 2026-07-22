package ingest

import (
	"context"
	"testing"

	"github.com/vollminlab/vollmint/internal/store"
)

func seedAccount(t *testing.T, s *store.Store, id, owner string) {
	t.Helper()
	if err := s.UpsertAccounts(context.Background(),
		[]store.Account{{ID: id, Name: id, Org: "test", Owner: owner}}); err != nil {
		t.Fatal(err)
	}
}

func seedFull(t *testing.T, s *store.Store, source, extID, acct, posted, amount, desc string) int64 {
	t.Helper()
	_, err := s.UpsertTransactions(context.Background(), []store.Txn{{
		Source: source, ExternalID: extID, AccountID: acct,
		Posted: day(posted), Amount: amount, Description: desc, Payee: desc,
	}})
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	s.Pool.QueryRow(context.Background(),
		`SELECT id FROM transactions WHERE source=$1 AND external_id=$2`, source, extID).Scan(&id)
	return id
}

func categoryOf(t *testing.T, s *store.Store, id int64) string {
	t.Helper()
	var name string
	s.Pool.QueryRow(context.Background(), `SELECT coalesce(c.name,'') FROM transactions t
		LEFT JOIN categories c ON c.id=t.category_id WHERE t.id=$1`, id).Scan(&name)
	return name
}

func peerOf(t *testing.T, s *store.Store, id int64) int64 {
	t.Helper()
	var peer *int64
	s.Pool.QueryRow(context.Background(),
		`SELECT transfer_peer_id FROM transactions WHERE id=$1`, id).Scan(&peer)
	if peer == nil {
		return 0
	}
	return *peer
}

func TestMatchVenmoPairs(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedAccount(t, s, "ally", "scott")

	bank := seedFull(t, s, "simplefin", "b1", "ally", "2026-07-16", "-32.00", "VENMO PAYMENT 4111")
	venmo := seedFull(t, s, "venmo_csv", "v1", "venmo", "2026-07-15", "-32.00", "Pizza night")
	lonely := seedFull(t, s, "simplefin", "b2", "ally", "2026-07-01", "-99.00", "VENMO PAYMENT 9999")

	n, err := MatchTransfers(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 pair, got %d", n)
	}
	if peerOf(t, s, bank) != venmo || peerOf(t, s, venmo) != bank {
		t.Error("peer ids not linked both ways")
	}
	if categoryOf(t, s, bank) != "Transfer" {
		t.Errorf("bank side category = %q, want Transfer", categoryOf(t, s, bank))
	}
	if categoryOf(t, s, venmo) == "Transfer" {
		t.Error("venmo side must keep its own category (it carries the spend)")
	}
	if peerOf(t, s, lonely) != 0 {
		t.Error("unmatched VENMO debit must stay unpaired (counted as spend)")
	}

	// Idempotency: second run pairs nothing new.
	if n2, _ := MatchTransfers(ctx, s); n2 != 0 {
		t.Fatalf("second run paired %d, want 0", n2)
	}
}

func TestMatchCardPaymentPairs(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedAccount(t, s, "chase-checking", "joint")
	seedAccount(t, s, "discover-card", "scott")

	out := seedFull(t, s, "simplefin", "c1", "chase-checking", "2026-07-10", "-500.00", "DISCOVER E-PAYMENT 1234")
	in := seedFull(t, s, "simplefin", "c2", "discover-card", "2026-07-12", "500.00", "DIRECTPAY PAYMENT THANK YOU")
	spend := seedFull(t, s, "simplefin", "c3", "discover-card", "2026-07-11", "-500.00", "BEST BUY 500")

	n, err := MatchTransfers(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 pair, got %d", n)
	}
	if peerOf(t, s, out) != in || peerOf(t, s, in) != out {
		t.Error("card payment pair not linked")
	}
	if categoryOf(t, s, out) != "Transfer" || categoryOf(t, s, in) != "Transfer" {
		t.Error("both card-payment sides must be Transfer")
	}
	if peerOf(t, s, spend) != 0 || categoryOf(t, s, spend) == "Transfer" {
		t.Error("ordinary card spend must not be swept into a transfer pair")
	}
}
