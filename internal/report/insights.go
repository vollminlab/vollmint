package report

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vollminlab/vollmint/internal/store"
)

// Insight is a single generated card surfaced to the user.
type Insight struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Amount string `json:"amount"`
}

// InsightCategorySpikes emits one card per category: budget breach when a
// budget is exceeded, else a spike card when spent >= 1.25x the 3-month
// average and at least $50 over it. Cap 5, sorted by delta descending.
func InsightCategorySpikes(ctx context.Context, s *store.Store, view, month string, now time.Time) ([]Insight, error) {
	own, args := ownerFilter(view, 2)
	full := append([]any{month + "-01"}, args...)

	rows, err := s.Pool.Query(ctx, `
WITH eff AS (
  SELECT COALESCE(sp.category_id, t.category_id) AS cat_id,
         COALESCE(sp.amount, t.amount) AS amt,
         date_trunc('month', t.posted)::date AS m
  FROM transactions t
  JOIN accounts a ON a.id = t.account_id
  LEFT JOIN transaction_splits sp ON sp.transaction_id = t.id
  WHERE t.amount < 0 AND t.transfer_peer_id IS NULL
    AND t.posted >= ($1::date - interval '3 months')
    AND t.posted < ($1::date + interval '1 month')`+own+`
),
cur AS (SELECT cat_id, -SUM(amt) AS spent FROM eff WHERE m = $1::date GROUP BY cat_id),
prev AS (SELECT cat_id, -SUM(amt)/3 AS avg3 FROM eff WHERE m < $1::date GROUP BY cat_id)
SELECT c.id, c.name, cur.spent::text, round(COALESCE(prev.avg3, 0), 2)::text,
       COALESCE(b.amount::text, '')
FROM cur
JOIN categories c ON c.id = cur.cat_id
LEFT JOIN prev ON prev.cat_id = cur.cat_id
LEFT JOIN budgets b ON b.category_id = c.id AND b.month = $1::date
WHERE c.kind <> 'transfer'
ORDER BY c.name`, full...)
	if err != nil {
		return nil, fmt.Errorf("category spikes: %w", err)
	}
	defer rows.Close()

	type carded struct {
		in    Insight
		delta int64
	}
	var cards []carded
	current := month == now.Format("2006-01")
	mt, _ := time.Parse("2006-01", month)
	daysIn := time.Date(mt.Year(), mt.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()

	for rows.Next() {
		var id int
		var name, spent, avg3, budget string
		if err := rows.Scan(&id, &name, &spent, &avg3, &budget); err != nil {
			return nil, fmt.Errorf("spike scan: %w", err)
		}
		spentC, avgC := cents(spent), cents(avg3)

		if budget != "" {
			budgetC := cents(budget)
			if spentC > budgetC {
				over := spentC - budgetC
				body := fmt.Sprintf("%s is %s over its %s budget.", name, usd(over), usd(budgetC))
				if current {
					left := daysIn - now.Day()
					body = fmt.Sprintf("%s is %s over its %s budget with %d days left in the month.",
						name, usd(over), usd(budgetC), left)
				}
				cards = append(cards, carded{Insight{
					Type: "budget_breach", Title: name + " is over budget",
					Body: body, Amount: centsToDec(over)}, over})
				continue
			}
		}
		if avgC > 0 && 4*spentC >= 5*avgC && spentC-avgC >= 5000 {
			delta := spentC - avgC
			cards = append(cards, carded{Insight{
				Type:  "category_spike",
				Title: name + " is running hot",
				Body: fmt.Sprintf("You've spent %s on %s this month — %s above your 3-month average of %s.",
					usd(spentC), name, usd(delta), usd(avgC)),
				Amount: centsToDec(delta)}, delta})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("spike rows: %w", err)
	}

	sort.SliceStable(cards, func(i, j int) bool { return cards[i].delta > cards[j].delta })
	if len(cards) > 5 {
		cards = cards[:5]
	}
	out := make([]Insight, len(cards))
	for i, c := range cards {
		out[i] = c.in
	}
	return out, nil
}

var overlapGroups = []struct {
	display string
	keys    []string
}{
	{"streaming", []string{"netflix", "hulu", "disney", "hbo max", "hbomax", "max.com", "paramount", "peacock", "youtube premium", "apple tv"}},
	{"music", []string{"spotify", "apple music", "tidal", "pandora"}},
	{"cloud storage", []string{"dropbox", "google one", "icloud", "onedrive"}},
	{"AI", []string{"anthropic", "claude.ai", "openai", "chatgpt"}},
}

// InsightSubscriptions audits recurring charges: total burn, price
// increases, and overlapping same-purpose services.
func InsightSubscriptions(ctx context.Context, s *store.Store, view, month string) ([]Insight, error) {
	own, args := ownerFilter(view, 2)
	full := append([]any{month + "-01"}, args...)

	rows, err := s.Pool.Query(ctx, cadenceCTEHead+own+cadenceCTETail+`,
ranked AS (
  SELECT payee, mag, posted,
         row_number() OVER (PARTITION BY payee ORDER BY posted DESC, id DESC) AS rn
  FROM hist
),
stats AS (
  SELECT payee, percentile_cont(0.5) WITHIN GROUP (ORDER BY mag) AS med
  FROM hist GROUP BY payee
),
catmode AS (
  SELECT payee, mode() WITHIN GROUP (ORDER BY category_id) AS category_id
  FROM hist WHERE category_id IS NOT NULL GROUP BY payee
)
SELECT cad.payee, COALESCE(c.name, ''), l.mag::text,
       COALESCE(p.mag::text, ''), round(st.med::numeric, 2)::text
FROM cadence cad
JOIN ranked l ON l.payee = cad.payee AND l.rn = 1
LEFT JOIN ranked p ON p.payee = cad.payee AND p.rn = 2
JOIN stats st ON st.payee = cad.payee
LEFT JOIN catmode cm ON cm.payee = cad.payee
LEFT JOIN categories c ON c.id = cm.category_id
ORDER BY cad.payee`, full...)
	if err != nil {
		return nil, fmt.Errorf("subscription audit: %w", err)
	}
	defer rows.Close()

	type sub struct {
		payee, category   string
		latC, prevC, medC int64
		hasPrev, subLike  bool
	}
	var all []sub
	for rows.Next() {
		var payee, category, latest, prev, med string
		if err := rows.Scan(&payee, &category, &latest, &prev, &med); err != nil {
			return nil, fmt.Errorf("subscription scan: %w", err)
		}
		sb := sub{payee: payee, category: category,
			latC: cents(latest), medC: cents(med), hasPrev: prev != ""}
		if sb.hasPrev {
			sb.prevC = cents(prev)
		}
		diff := sb.latC - sb.medC
		if diff < 0 {
			diff = -diff
		}
		sb.subLike = category == "Subscriptions" || diff*10 <= sb.medC
		all = append(all, sb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("subscription rows: %w", err)
	}

	var out []Insight

	// Total burn over subscription-like payees.
	var subs []sub
	var totalC int64
	for _, sb := range all {
		if sb.subLike {
			subs = append(subs, sb)
			totalC += sb.latC
		}
	}
	if len(subs) > 0 {
		top := make([]sub, len(subs))
		copy(top, subs)
		sort.SliceStable(top, func(i, j int) bool { return top[i].latC > top[j].latC })
		if len(top) > 3 {
			top = top[:3]
		}
		var names []string
		for _, sb := range top {
			names = append(names, fmt.Sprintf("%s (%s)", titleCase(sb.payee), usd(sb.latC)))
		}
		noun := "recurring charges"
		if len(subs) == 1 {
			noun = "recurring charge"
		}
		out = append(out, Insight{
			Type:  "subscription_total",
			Title: "Recurring charges add up",
			Body: fmt.Sprintf("You're carrying %s/month across %d %s. Largest: %s.",
				usd(totalC), len(subs), noun, strings.Join(names, ", ")),
			Amount: centsToDec(totalC),
		})
	}

	// Price increases — any cadence payee, not just sub-like.
	for _, sb := range all {
		if !sb.hasPrev || sb.latC <= sb.prevC {
			continue
		}
		diff := sb.latC - sb.prevC
		if diff*20 > sb.prevC && diff >= 100 {
			out = append(out, Insight{
				Type:  "price_increase",
				Title: titleCase(sb.payee) + " price went up",
				Body: fmt.Sprintf("%s went from %s to %s (+%s).",
					titleCase(sb.payee), usd(sb.prevC), usd(sb.latC), usd(diff)),
				Amount: centsToDec(diff),
			})
		}
	}

	// Overlaps within the subscription-like set.
	for _, g := range overlapGroups {
		var matched []sub
		for _, sb := range subs {
			low := strings.ToLower(sb.payee)
			for _, k := range g.keys {
				if strings.Contains(low, k) {
					matched = append(matched, sb)
					break
				}
			}
		}
		if len(matched) >= 2 {
			var names []string
			var sumC int64
			for _, m := range matched {
				names = append(names, titleCase(m.payee))
				sumC += m.latC
			}
			out = append(out, Insight{
				Type:  "subscription_overlap",
				Title: "Overlapping " + g.display + " subscriptions",
				Body: fmt.Sprintf("You're paying for %d %s services (%s) — %s/month combined.",
					len(matched), g.display, strings.Join(names, ", "), usd(sumC)),
				Amount: centsToDec(sumC),
			})
		}
	}

	return out, nil
}

// Insights combines all generators, sorted by money at stake descending.
func Insights(ctx context.Context, s *store.Store, view, month string, now time.Time) ([]Insight, error) {
	spikes, err := InsightCategorySpikes(ctx, s, view, month, now)
	if err != nil {
		return nil, err
	}
	subs, err := InsightSubscriptions(ctx, s, view, month)
	if err != nil {
		return nil, err
	}
	all := append(spikes, subs...)
	sort.SliceStable(all, func(i, j int) bool {
		return cents(all[i].Amount) > cents(all[j].Amount)
	})
	return all, nil
}
