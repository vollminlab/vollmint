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
