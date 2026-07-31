package report

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestInsightCategorySpike(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// 3-month Dining average = $100; July = $200 → 2.0x and +$100 → fires.
	seedSpend(t, s, "acct-ins", "scott", "d-apr", "2026-04-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-ins", "scott", "d-may", "2026-05-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-ins", "scott", "d-jun", "2026-06-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-ins", "scott", "d-jul", "2026-07-10", "-200.00", "Dining")

	// Groceries: flat — must NOT fire.
	seedSpend(t, s, "acct-ins", "scott", "g-apr", "2026-04-11", "-80.00", "Groceries")
	seedSpend(t, s, "acct-ins", "scott", "g-may", "2026-05-11", "-80.00", "Groceries")
	seedSpend(t, s, "acct-ins", "scott", "g-jun", "2026-06-11", "-80.00", "Groceries")
	seedSpend(t, s, "acct-ins", "scott", "g-jul", "2026-07-11", "-82.00", "Groceries")

	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC) // viewing a past month
	items, err := InsightCategorySpikes(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("spikes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 insight, got %+v", items)
	}
	in := items[0]
	if in.Type != "category_spike" || in.Title != "Dining is running hot" {
		t.Fatalf("wrong card: %+v", in)
	}
	if !strings.Contains(in.Body, "$200.00") || !strings.Contains(in.Body, "$100.00") {
		t.Fatalf("body missing amounts: %q", in.Body)
	}
	if in.Amount != "100.00" {
		t.Fatalf("amount %q, want delta 100.00", in.Amount)
	}
}

func TestInsightBudgetBreachBeatsSpike(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seedSpend(t, s, "acct-ins", "scott", "b-apr", "2026-04-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-ins", "scott", "b-may", "2026-05-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-ins", "scott", "b-jun", "2026-06-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-ins", "scott", "b-jul", "2026-07-10", "-200.00", "Dining")
	setBudget(t, s, "Dining", "2026-07", "150.00")

	// Current month: body mentions days left.
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	items, err := InsightCategorySpikes(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("spikes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("breach must replace spike (one card per category): %+v", items)
	}
	in := items[0]
	if in.Type != "budget_breach" || in.Title != "Dining is over budget" {
		t.Fatalf("wrong card: %+v", in)
	}
	if !strings.Contains(in.Body, "$50.00 over") || !strings.Contains(in.Body, "$150.00 budget") {
		t.Fatalf("body: %q", in.Body)
	}
	if !strings.Contains(in.Body, "11 days left") { // July has 31 days; 31-20=11
		t.Fatalf("current-month body must count days left: %q", in.Body)
	}
	if in.Amount != "50.00" {
		t.Fatalf("amount %q, want overage 50.00", in.Amount)
	}

	// Past month: no days-left clause.
	past := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	items, err = InsightCategorySpikes(ctx, s, "household", "2026-07", past)
	if err != nil {
		t.Fatalf("spikes past: %v", err)
	}
	if strings.Contains(items[0].Body, "days left") {
		t.Fatalf("past month must not mention days left: %q", items[0].Body)
	}
}

func TestInsightSubscriptions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Netflix: price increase 15.49 → 17.99 (+16%, +$2.50). Streaming.
	seedBill(t, s, "acct-sub", "scott", "nx-apr", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), "-15.49", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-sub", "scott", "nx-may", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), "-15.49", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-sub", "scott", "nx-jun", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX", "Subscriptions")

	// Hulu: stable. Streaming → overlap with Netflix.
	seedBill(t, s, "acct-sub", "scott", "hl-apr", time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC), "-15.49", "HULU", "Subscriptions")
	seedBill(t, s, "acct-sub", "scott", "hl-may", time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC), "-15.49", "HULU", "Subscriptions")
	seedBill(t, s, "acct-sub", "scott", "hl-jun", time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), "-15.49", "HULU", "Subscriptions")

	items, err := InsightSubscriptions(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatalf("subs: %v", err)
	}
	byType := map[string]Insight{}
	for _, in := range items {
		byType[in.Type] = in
	}

	total, ok := byType["subscription_total"]
	if !ok {
		t.Fatalf("missing subscription_total: %+v", items)
	}
	if !strings.Contains(total.Body, "$33.48/month") || !strings.Contains(total.Body, "2 recurring") {
		t.Fatalf("total body: %q", total.Body)
	}

	inc, ok := byType["price_increase"]
	if !ok {
		t.Fatalf("missing price_increase: %+v", items)
	}
	if inc.Title != "Netflix price went up" ||
		!strings.Contains(inc.Body, "$15.49") || !strings.Contains(inc.Body, "$17.99") ||
		!strings.Contains(inc.Body, "+$2.50") {
		t.Fatalf("increase card: %+v", inc)
	}
	if inc.Amount != "2.50" {
		t.Fatalf("increase amount %q", inc.Amount)
	}

	ovl, ok := byType["subscription_overlap"]
	if !ok {
		t.Fatalf("missing subscription_overlap: %+v", items)
	}
	if ovl.Title != "Overlapping streaming subscriptions" ||
		!strings.Contains(ovl.Body, "2 streaming services") ||
		!strings.Contains(ovl.Body, "Netflix") || !strings.Contains(ovl.Body, "Hulu") {
		t.Fatalf("overlap card: %+v", ovl)
	}
}
