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

// TestInsightBudgetBreachLastDayPhrasing covers the current-month body on
// the last day of the month ("0 days left" would read wrong) and the
// singular "1 day left" case the day before.
func TestInsightBudgetBreachLastDayPhrasing(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seedSpend(t, s, "acct-ins", "scott", "ld-jul", "2026-07-10", "-200.00", "Dining")
	setBudget(t, s, "Dining", "2026-07", "150.00")

	// July 31 — last day of the month.
	now := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
	items, err := InsightCategorySpikes(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("spikes: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 breach card, got %+v", items)
	}
	if !strings.Contains(items[0].Body, "on the last day of the month") {
		t.Fatalf("last-day body: %q", items[0].Body)
	}
	if strings.Contains(items[0].Body, "days left") {
		t.Fatalf("last-day body must not say days left: %q", items[0].Body)
	}

	// July 30 — exactly one day left, singular.
	now = time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	items, err = InsightCategorySpikes(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("spikes: %v", err)
	}
	if !strings.Contains(items[0].Body, "with 1 day left in the month") {
		t.Fatalf("one-day body: %q", items[0].Body)
	}
}

// TestInsightCategorySpikeShortHistory guards the average divisor: with only
// one prior month of history, the baseline is that month's spend — not
// spend/3. A $100 → $110 month-over-month change must not read as a spike.
func TestInsightCategorySpikeShortHistory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seedSpend(t, s, "acct-short", "scott", "sh-jun", "2026-06-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-short", "scott", "sh-jul", "2026-07-10", "-110.00", "Dining")

	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	items, err := InsightCategorySpikes(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("spikes: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("short history must average over months present (avg=100, not 33.33): %+v", items)
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

// TestInsightSubscriptionsSameDayTiebreak mirrors
// TestForecastSameDayTiebreak (forecast_test.go): a payee with a qualifying
// cadence whose latest month has two same-day charges with different
// amounts. The `ranked` CTE's row_number() must deterministically pick the
// higher-id (later-inserted) row as "latest" — without the `id DESC`
// tiebreak, ties on posted date are resolved arbitrarily by the planner and
// the price-increase amount would flap between runs.
func TestInsightSubscriptionsSameDayTiebreak(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seedBill(t, s, "acct-tie", "scott", "dc-apr", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), "-20.00", "DOUBLE CHARGE SUB", "Subscriptions")
	seedBill(t, s, "acct-tie", "scott", "dc-may", time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), "-20.00", "DOUBLE CHARGE SUB", "Subscriptions")
	// Same calendar day, two charges — dc-jun-a inserted first (lower id),
	// dc-jun-b inserted second (higher id). rn=1 must be dc-jun-b.
	seedBill(t, s, "acct-tie", "scott", "dc-jun-a", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "-20.00", "DOUBLE CHARGE SUB", "Subscriptions")
	seedBill(t, s, "acct-tie", "scott", "dc-jun-b", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "-30.00", "DOUBLE CHARGE SUB", "Subscriptions")

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
	if !strings.Contains(total.Body, "$30.00/month") {
		t.Fatalf("total must reflect higher-id (later-inserted) same-day charge ($30.00): %q", total.Body)
	}

	inc, ok := byType["price_increase"]
	if !ok {
		t.Fatalf("missing price_increase: %+v", items)
	}
	if !strings.Contains(inc.Body, "$20.00") || !strings.Contains(inc.Body, "$30.00") ||
		!strings.Contains(inc.Body, "+$10.00") {
		t.Fatalf("increase card must compare rn=2 ($20.00, dc-jun-a) to rn=1 ($30.00, dc-jun-b): %q", inc.Body)
	}
	if inc.Amount != "10.00" {
		t.Fatalf("increase amount %q, want 10.00 (deterministic higher-id tiebreak)", inc.Amount)
	}
}

// TestInsightSubscriptionOverlapExcludesMaxFalsePositive guards against the
// bare "max" keyword matching non-streaming payees like "MAX FITNESS" (or
// "TJ MAXX", "AUTOZONE MAX", etc.) via substring match. The streaming group
// must only match on specific HBO Max tokens.
func TestInsightSubscriptionOverlapExcludesMaxFalsePositive(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	seedBill(t, s, "acct-max", "scott", "nx-apr", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), "-15.99", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-max", "scott", "nx-may", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), "-15.99", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-max", "scott", "nx-jun", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), "-15.99", "NETFLIX", "Subscriptions")

	seedBill(t, s, "acct-max", "scott", "hl-apr", time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC), "-15.49", "HULU", "Subscriptions")
	seedBill(t, s, "acct-max", "scott", "hl-may", time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC), "-15.49", "HULU", "Subscriptions")
	seedBill(t, s, "acct-max", "scott", "hl-jun", time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC), "-15.49", "HULU", "Subscriptions")

	// Recurring, stable-priced gym membership that must NOT be swept into
	// the "streaming" overlap group by a bare "max" substring match.
	seedBill(t, s, "acct-max", "scott", "mf-apr", time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC), "-40.00", "MAX FITNESS", "Subscriptions")
	seedBill(t, s, "acct-max", "scott", "mf-may", time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC), "-40.00", "MAX FITNESS", "Subscriptions")
	seedBill(t, s, "acct-max", "scott", "mf-jun", time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC), "-40.00", "MAX FITNESS", "Subscriptions")

	items, err := InsightSubscriptions(ctx, s, "household", "2026-07")
	if err != nil {
		t.Fatalf("subs: %v", err)
	}
	var ovl *Insight
	for i, in := range items {
		if in.Type == "subscription_overlap" {
			ovl = &items[i]
		}
	}
	if ovl == nil {
		t.Fatalf("missing subscription_overlap: %+v", items)
	}
	if ovl.Title != "Overlapping streaming subscriptions" ||
		!strings.Contains(ovl.Body, "2 streaming services") ||
		!strings.Contains(ovl.Body, "Netflix") || !strings.Contains(ovl.Body, "Hulu") {
		t.Fatalf("overlap card: %+v", ovl)
	}
	if strings.Contains(ovl.Body, "Max Fitness") {
		t.Fatalf("Max Fitness must not be swept into streaming overlap: %q", ovl.Body)
	}
}

// TestInsightCategorySpikeZeroBaselineGuard ensures a category with no
// spend in the prior 3 months (avg3 == 0) does not trivially satisfy the
// "spent >= 1.25x average" gate on its first-ever month of spend.
func TestInsightCategorySpikeZeroBaselineGuard(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Health has zero spend history — this is its first-ever charge, well
	// above the $50 floor, but there is no baseline to compare against.
	seedSpend(t, s, "acct-zero", "scott", "h-jul", "2026-07-05", "-75.00", "Health")

	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	items, err := InsightCategorySpikes(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("spikes: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("zero-baseline category must not spike: %+v", items)
	}
}

// TestInsightsCombinerSmoke is a light smoke test for the Insights
// combiner: it merges both generators and sorts by money at stake.
func TestInsightsCombinerSmoke(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Dining category spike — delta $100.
	seedSpend(t, s, "acct-comb", "scott", "dc-apr", "2026-04-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-comb", "scott", "dc-may", "2026-05-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-comb", "scott", "dc-jun", "2026-06-10", "-100.00", "Dining")
	seedSpend(t, s, "acct-comb", "scott", "dc-jul", "2026-07-10", "-200.00", "Dining")

	// Netflix price increase — $2.50, much smaller money at stake.
	seedBill(t, s, "acct-comb", "scott", "nx-apr", time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), "-15.49", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-comb", "scott", "nx-may", time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), "-15.49", "NETFLIX", "Subscriptions")
	seedBill(t, s, "acct-comb", "scott", "nx-jun", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), "-17.99", "NETFLIX", "Subscriptions")

	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	items, err := Insights(ctx, s, "household", "2026-07", now)
	if err != nil {
		t.Fatalf("Insights: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("want at least 2 combined insights, got %+v", items)
	}
	for i := 1; i < len(items); i++ {
		if cents(items[i-1].Amount) < cents(items[i].Amount) {
			t.Fatalf("not sorted descending by amount: %+v", items)
		}
	}
	if items[0].Type != "category_spike" {
		t.Fatalf("want the $100 Dining spike to outrank smaller-dollar cards, got %+v", items[0])
	}
}
