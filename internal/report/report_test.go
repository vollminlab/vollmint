package report

import (
	"context"
	"testing"
)

func TestSummaryTotals(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	// July spend + income + a vice + a transfer that must be excluded
	seedSpend(t, s, "ally-s", "scott", "sp1", "2026-07-05", "-100.00", "Groceries")
	seedSpend(t, s, "ally-s", "scott", "sp2", "2026-07-06", "-40.00", "Dining") // Dining is a vice
	seedSpend(t, s, "ally-s", "scott", "in1", "2026-07-01", "3000.00", "Paycheck")
	seedSpend(t, s, "ally-s", "scott", "tr1", "2026-07-07", "-500.00", "Transfer") // excluded
	setBudget(t, s, "Groceries", "2026-07", "120.00")

	sum, err := Summary(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if sum.In != "3000.00" {
		t.Errorf("In = %q, want 3000.00", sum.In)
	}
	if sum.Out != "140.00" { // 100 + 40, transfer excluded
		t.Errorf("Out = %q, want 140.00", sum.Out)
	}
	if sum.Vices != "40.00" { // Dining is_vice
		t.Errorf("Vices = %q, want 40.00", sum.Vices)
	}
	if sum.BudgetTotal != "120.00" {
		t.Errorf("BudgetTotal = %q, want 120.00", sum.BudgetTotal)
	}
}

func TestSpendByCategory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	seedSpend(t, s, "ally-s", "scott", "g1", "2026-07-05", "-60.00", "Groceries")
	seedSpend(t, s, "ally-s", "scott", "g2", "2026-07-08", "-40.00", "Groceries")
	seedSpend(t, s, "ally-s", "scott", "d1", "2026-07-09", "-25.00", "Dining")
	setBudget(t, s, "Groceries", "2026-07", "120.00")

	rows, err := SpendByCategory(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	// Groceries 100.00 (budget 120.00) should sort before Dining 25.00
	if len(rows) != 2 {
		t.Fatalf("got %d category rows, want 2", len(rows))
	}
	if rows[0].Category != "Groceries" || rows[0].Spent != "100.00" || rows[0].Budget != "120.00" {
		t.Errorf("row0 = %+v", rows[0])
	}
	if rows[1].Category != "Dining" || rows[1].Budget != "" {
		t.Errorf("row1 = %+v (Dining should have empty budget)", rows[1])
	}
}

func TestSummaryViewFilter(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	// Seed spends for scott and nikki in July
	seedSpend(t, s, "ally-s", "scott", "vf1", "2026-07-05", "-50.00", "Groceries")
	seedSpend(t, s, "ally-n", "nikki", "vf2", "2026-07-06", "-30.00", "Groceries")
	// Seed a third txn under scott's account, then override the owner to nikki
	seedSpend(t, s, "ally-s", "scott", "vf3", "2026-07-07", "-20.00", "Dining")
	if _, err := s.Pool.Exec(ctx, `UPDATE transactions SET owner_override='nikki' WHERE source='simplefin' AND external_id='vf3'`); err != nil {
		t.Fatal(err)
	}

	// scott's view should only see vf1 (50.00), vf3 is overridden away
	sum, err := Summary(ctx, s, "scott", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Out != "50.00" {
		t.Errorf("scott summary Out = %q, want 50.00", sum.Out)
	}

	// nikki's view should see vf2 (30) + overridden vf3 (20) = 50.00
	sum, err = Summary(ctx, s, "nikki", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Out != "50.00" {
		t.Errorf("nikki summary Out = %q, want 50.00", sum.Out)
	}

	// household view should see all three = 100.00
	sum, err = Summary(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Out != "100.00" {
		t.Errorf("household summary Out = %q, want 100.00", sum.Out)
	}

	// SpendByCategory for scott should have only 1 row (Groceries, 50.00)
	rows, err := SpendByCategory(ctx, s, "scott", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("scott SpendByCategory got %d rows, want 1", len(rows))
	}
	if rows[0].Category != "Groceries" || rows[0].Spent != "50.00" {
		t.Errorf("scott SpendByCategory row0 = %+v", rows[0])
	}
}

func TestSummaryMonthBoundary(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	// Seed spends at month boundaries: June 30, July 1, July 31, Aug 1
	seedSpend(t, s, "ally-s", "scott", "mb1", "2026-06-30", "-10.00", "Groceries")
	seedSpend(t, s, "ally-s", "scott", "mb2", "2026-07-01", "-40.00", "Groceries")
	seedSpend(t, s, "ally-s", "scott", "mb3", "2026-07-31", "-25.00", "Groceries")
	seedSpend(t, s, "ally-s", "scott", "mb4", "2026-08-01", "-15.00", "Groceries")

	// July should include July 1 and July 31 only (half-open: >= 2026-07-01 and < 2026-08-01)
	sum, err := Summary(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Out != "65.00" {
		t.Errorf("July summary Out = %q, want 65.00", sum.Out)
	}

	// SpendByCategory for July should have 1 row with Spent 65.00
	rows, err := SpendByCategory(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("July SpendByCategory got %d rows, want 1", len(rows))
	}
	if rows[0].Category != "Groceries" || rows[0].Spent != "65.00" {
		t.Errorf("July SpendByCategory row0 = %+v", rows[0])
	}
}
