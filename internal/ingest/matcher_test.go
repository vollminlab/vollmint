package ingest

import (
	"context"
	"fmt"
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
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM transactions WHERE source=$1 AND external_id=$2`, source, extID).Scan(&id); err != nil {
		t.Fatalf("seedFull lookup: %v", err)
	}
	return id
}

func categoryOf(t *testing.T, s *store.Store, id int64) string {
	t.Helper()
	var name string
	if err := s.Pool.QueryRow(context.Background(), `SELECT coalesce(c.name,'') FROM transactions t
		LEFT JOIN categories c ON c.id=t.category_id WHERE t.id=$1`, id).Scan(&name); err != nil {
		t.Fatalf("categoryOf: %v", err)
	}
	return name
}

func setCategory(t *testing.T, s *store.Store, id int64, name string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(),
		`UPDATE transactions SET category_id=(SELECT id FROM categories WHERE name=$1) WHERE id=$2`,
		name, id); err != nil {
		t.Fatalf("setCategory: %v", err)
	}
}

func peerOf(t *testing.T, s *store.Store, id int64) int64 {
	t.Helper()
	var peer *int64
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT transfer_peer_id FROM transactions WHERE id=$1`, id).Scan(&peer); err != nil {
		t.Fatalf("peerOf: %v", err)
	}
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

// seedVenmoTyped seeds a venmo_csv row with a raw {"type": ...} payload, the
// way the Venmo CSV parser records the statement's Type column.
func seedVenmoTyped(t *testing.T, s *store.Store, extID, posted, amount, note, typ string) int64 {
	t.Helper()
	_, err := s.UpsertTransactions(context.Background(), []store.Txn{{
		Source: "venmo_csv", ExternalID: extID, AccountID: "venmo",
		Posted: day(posted), Amount: amount, Description: note, Payee: note,
		Raw: []byte(fmt.Sprintf(`{"type":%q}`, typ)),
	}})
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := s.Pool.QueryRow(context.Background(),
		`SELECT id FROM transactions WHERE source='venmo_csv' AND external_id=$1`, extID).Scan(&id); err != nil {
		t.Fatalf("seedVenmoTyped lookup: %v", err)
	}
	return id
}

func TestMatchVenmoCashoutPairs(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedAccount(t, s, "ally", "scott")

	// Cashout: Venmo balance → bank. Bank-side credit + venmo Standard Transfer.
	bank := seedFull(t, s, "simplefin", "b1", "ally", "2026-05-18", "431.00", "VENMO CASHOUT")
	venmo := seedVenmoTyped(t, s, "v1", "2026-05-17", "-431.00", "", "Standard Transfer")

	// Decoy: an ordinary venmo spend of matching magnitude must not be swept
	// into a cashout pair — only Standard Transfer rows qualify.
	bank2 := seedFull(t, s, "simplefin", "b2", "ally", "2026-05-27", "30.00", "VENMO CASHOUT")
	spend := seedVenmoTyped(t, s, "v2", "2026-05-26", "-30.00", "Thank you", "Payment")

	n, err := MatchTransfers(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 pair, got %d", n)
	}
	if peerOf(t, s, bank) != venmo || peerOf(t, s, venmo) != bank {
		t.Error("cashout peer ids not linked both ways")
	}
	if categoryOf(t, s, bank) != "Transfer" || categoryOf(t, s, venmo) != "Transfer" {
		t.Errorf("both cashout legs must be Transfer, got bank=%q venmo=%q",
			categoryOf(t, s, bank), categoryOf(t, s, venmo))
	}
	if peerOf(t, s, bank2) != 0 || peerOf(t, s, spend) != 0 {
		t.Error("non-transfer venmo row must not pair with a bank credit")
	}
	if categoryOf(t, s, spend) == "Transfer" {
		t.Error("venmo spend row must keep its own category")
	}

	// Idempotency: second run pairs nothing new.
	if n2, _ := MatchTransfers(ctx, s); n2 != 0 {
		t.Fatalf("second run paired %d, want 0", n2)
	}
}

func TestMatchPreservesManualCategory(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedAccount(t, s, "ally", "scott")

	// Pair 1: bank leg carries a user-assigned category — must survive pairing.
	bank1 := seedFull(t, s, "simplefin", "b1", "ally", "2026-07-16", "-41.00", "VENMO PAYMENT 4111")
	venmo1 := seedFull(t, s, "venmo_csv", "v1", "venmo", "2026-07-15", "-41.00", "Pizza night")
	setCategory(t, s, bank1, "Groceries")

	// Pair 2: bank leg carries the seed placeholder — pairing may replace it.
	// Amounts differ so the two pairs cannot cross-match.
	bank2 := seedFull(t, s, "simplefin", "b2", "ally", "2026-07-16", "-52.00", "VENMO PAYMENT 4222")
	venmo2 := seedFull(t, s, "venmo_csv", "v2", "venmo", "2026-07-15", "-52.00", "Concert tickets")
	setCategory(t, s, bank2, "Needs Venmo detail")

	n, err := MatchTransfers(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want 2 pairs, got %d", n)
	}
	if peerOf(t, s, bank1) != venmo1 || peerOf(t, s, venmo1) != bank1 {
		t.Error("pair 1 peer ids not linked both ways")
	}
	if peerOf(t, s, bank2) != venmo2 || peerOf(t, s, venmo2) != bank2 {
		t.Error("pair 2 peer ids not linked both ways")
	}
	if got := categoryOf(t, s, bank1); got != "Groceries" {
		t.Errorf("manual category clobbered: got %q, want Groceries", got)
	}
	if got := categoryOf(t, s, bank2); got != "Transfer" {
		t.Errorf("placeholder category: got %q, want Transfer", got)
	}
}

func TestMatchSkipsSameAccountLegs(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	seedAccount(t, s, "chase-checking", "joint")

	out := seedFull(t, s, "simplefin", "s1", "chase-checking", "2026-07-10", "-500.00", "DISCOVER E-PAYMENT 1234")
	in := seedFull(t, s, "simplefin", "s2", "chase-checking", "2026-07-11", "500.00", "DIRECTPAY PAYMENT THANK YOU")

	n, err := MatchTransfers(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("want 0 pairs for same-account legs, got %d", n)
	}
	if peerOf(t, s, out) != 0 || peerOf(t, s, in) != 0 {
		t.Error("same-account legs must stay unpaired")
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

	// Idempotency: second run pairs nothing new.
	if n2, _ := MatchTransfers(ctx, s); n2 != 0 {
		t.Fatalf("second run paired %d, want 0", n2)
	}
}
