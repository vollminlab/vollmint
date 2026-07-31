// Package report holds read-only aggregation queries for the vollmint API.
// All money is summed in Postgres numeric and returned as decimal strings;
// no float64 ever touches a monetary value.
package report

import (
	"context"
	"fmt"

	"github.com/vollminlab/vollmint/internal/store"
)

// ownerFilter returns a SQL fragment filtering on the effective owner
// (COALESCE(t.owner_override, a.owner)) for scott/nikki/joint views, using
// placeholder $argN. The household view applies no filter.
func ownerFilter(view string, argN int) (string, []any) {
	switch view {
	case "scott", "nikki", "joint":
		return fmt.Sprintf(" AND COALESCE(t.owner_override, a.owner) = $%d", argN), []any{view}
	}
	return "", nil
}

// notTransfer excludes transfer rows from spend/income math.
const notTransfer = ` AND t.transfer_peer_id IS NULL
	AND (c.kind IS NULL OR c.kind <> 'transfer')`

// cadenceCTEHead opens the shared "spend" CTE used by both Forecast and the
// subscription insights: non-transfer, non-P2P (Venmo/Zelle excluded)
// payee-grouped spend joined to accounts. It selects t.id so callers can
// build deterministic same-day tiebreaks (ORDER BY posted DESC, id DESC) —
// without it, ties on posted date are resolved arbitrarily by the planner.
// A caller must concatenate an ownerFilter fragment immediately after this
// constant, then close out with cadenceCTETail. Example:
//
//	own, args := ownerFilter(view, 2)
//	q := cadenceCTEHead + own + cadenceCTETail + `, ... rest of query`
const cadenceCTEHead = `
WITH spend AS (
  SELECT t.id, t.payee, -t.amount AS mag, t.posted, t.pending,
         date_trunc('month', t.posted)::date AS m, t.category_id
  FROM transactions t
  JOIN accounts a ON a.id = t.account_id
  LEFT JOIN categories c ON c.id = t.category_id
  WHERE t.amount < 0 AND t.payee <> ''
    AND t.transfer_peer_id IS NULL
    AND (c.kind IS NULL OR c.kind <> 'transfer')
    AND t.payee NOT ILIKE '%venmo%' AND t.payee NOT ILIKE '%zelle%'`

// cadenceCTETail closes the "spend" CTE opened by cadenceCTEHead and adds
// the shared "hist" (non-pending, posted before the target month) and
// "cadence" (>=3 distinct months overall AND >=2 of the last 3 months —
// the recent-cadence gate that excludes payees that have gone dead) CTEs.
// Callers append further CTEs/SELECT referencing spend/hist/cadence.
const cadenceCTETail = `
),
hist AS (
  SELECT * FROM spend
  WHERE NOT pending AND posted < ($1::date + interval '1 month')
),
cadence AS (
  SELECT payee FROM hist GROUP BY payee
  HAVING count(DISTINCT m) >= 3
     AND count(DISTINCT m) FILTER (
           WHERE m >= ($1::date - interval '3 months') AND m < $1::date) >= 2
)`

// SummaryResult is the dashboard rollup. All amounts are decimal strings.
type SummaryResult struct {
	In          string `json:"in"`
	Out         string `json:"out"`
	Vices       string `json:"vices"`
	BudgetTotal string `json:"budget_total"`
	Month       string `json:"month"`
	View        string `json:"view"`
}

// Summary computes In/Out/Vices for the month+view and the month's total budget.
func Summary(ctx context.Context, s *store.Store, view, month string) (SummaryResult, error) {
	res := SummaryResult{In: "0.00", Out: "0.00", Vices: "0.00", BudgetTotal: "0.00", Month: month, View: view}
	own, args := ownerFilter(view, 2)
	full := append([]any{month + "-01"}, args...)

	// In/Out are raw transaction totals — split parts sum to the parent
	// amount, so totals are unaffected by splits.
	q := `
		SELECT
		  COALESCE(SUM(t.amount) FILTER (WHERE t.amount > 0), 0)::text,
		  COALESCE(-SUM(t.amount) FILTER (WHERE t.amount < 0), 0)::text
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.posted >= $1::date AND t.posted < ($1::date + interval '1 month')` +
		notTransfer + own
	if err := s.Pool.QueryRow(ctx, q, full...).Scan(&res.In, &res.Out); err != nil {
		return res, fmt.Errorf("summary totals: %w", err)
	}

	// Vices is split-aware: a split part is attributed to its own category,
	// so a split transaction only counts toward Vices for the parts whose
	// category is a vice.
	vq := `
		SELECT COALESCE(-SUM(COALESCE(sp.amount, t.amount)), 0)::text
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN transaction_splits sp ON sp.transaction_id = t.id
		JOIN categories c ON c.id = COALESCE(sp.category_id, t.category_id)
		WHERE c.is_vice AND t.amount < 0
		  AND t.posted >= $1::date AND t.posted < ($1::date + interval '1 month')
		  AND t.transfer_peer_id IS NULL AND c.kind <> 'transfer'` + own
	if err := s.Pool.QueryRow(ctx, vq, full...).Scan(&res.Vices); err != nil {
		return res, fmt.Errorf("summary vices: %w", err)
	}

	// Total budget for the month (view-independent — budgets are household).
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM budgets WHERE month = $1::date`,
		month+"-01").Scan(&res.BudgetTotal); err != nil {
		return res, fmt.Errorf("summary budget: %w", err)
	}
	return res, nil
}

// RecurringItem is a detected recurring charge.
type RecurringItem struct {
	Payee     string `json:"payee"`
	Count     int    `json:"count"`
	Months    int    `json:"months"`
	AvgAmount string `json:"avg_amount"`
	LastSeen  string `json:"last_seen"`
	FirstSeen string `json:"first_seen"`
	IsNew     bool   `json:"is_new"`
}

// CategorySpend is one row of the spend-by-category report.
type CategorySpend struct {
	CategoryID int    `json:"category_id"`
	Category   string `json:"category"`
	Spent      string `json:"spent"`
	Budget     string `json:"budget"` // "" when no budget set
	IsVice     bool   `json:"is_vice"`
}

// SpendByCategory returns spend per category for the month+view, descending by
// spent, with each category's budget (if any) for that month.
func SpendByCategory(ctx context.Context, s *store.Store, view, month string) ([]CategorySpend, error) {
	own, args := ownerFilter(view, 2)
	q := `
		SELECT c.id, c.name, (-SUM(COALESCE(sp.amount, t.amount)))::text, c.is_vice,
		       COALESCE(b.amount::text, '')
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN transaction_splits sp ON sp.transaction_id = t.id
		JOIN categories c ON c.id = COALESCE(sp.category_id, t.category_id)
		LEFT JOIN budgets b ON b.category_id = c.id AND b.month = $1::date
		WHERE t.amount < 0
		  AND t.posted >= $1::date AND t.posted < ($1::date + interval '1 month')
		  AND t.transfer_peer_id IS NULL AND c.kind <> 'transfer'` + own + `
		GROUP BY c.id, c.name, c.is_vice, b.amount
		ORDER BY (-SUM(COALESCE(sp.amount, t.amount))) DESC, c.name`
	full := append([]any{month + "-01"}, args...)
	rows, err := s.Pool.Query(ctx, q, full...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CategorySpend, 0)
	for rows.Next() {
		var cs CategorySpend
		if err := rows.Scan(&cs.CategoryID, &cs.Category, &cs.Spent, &cs.IsVice, &cs.Budget); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// Recurring detects recurring charges: payees with spend in >=3 distinct
// months. is_new flags payees whose first charge is within the given month.
// view filters by effective owner; the month only affects the is_new flag and
// is NOT a spend filter (recurrence is judged across all history).
func Recurring(ctx context.Context, s *store.Store, view, month string) ([]RecurringItem, error) {
	own, args := ownerFilter(view, 2)
	q := `
		WITH spend AS (
		  SELECT t.payee, -t.amount AS mag, t.posted,
		         date_trunc('month', t.posted) AS m
		  FROM transactions t
		  JOIN accounts a ON a.id = t.account_id
		  LEFT JOIN categories c ON c.id = t.category_id
		  WHERE t.amount < 0 AND t.payee <> ''
		    AND t.transfer_peer_id IS NULL
		    AND (c.kind IS NULL OR c.kind <> 'transfer')` + own + `
		)
		SELECT payee, count(*)::int, count(DISTINCT m)::int,
		       round(avg(mag), 2)::text,
		       to_char(max(posted),'YYYY-MM-DD'),
		       to_char(min(posted),'YYYY-MM-DD'),
		       (min(posted) >= $1::date AND min(posted) < ($1::date + interval '1 month')) AS is_new
		FROM spend
		GROUP BY payee
		HAVING count(DISTINCT m) >= 3
		ORDER BY avg(mag) DESC, payee`
	full := append([]any{month + "-01"}, args...)
	rows, err := s.Pool.Query(ctx, q, full...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecurringItem, 0)
	for rows.Next() {
		var ri RecurringItem
		if err := rows.Scan(&ri.Payee, &ri.Count, &ri.Months, &ri.AvgAmount, &ri.LastSeen, &ri.FirstSeen, &ri.IsNew); err != nil {
			return nil, err
		}
		out = append(out, ri)
	}
	return out, rows.Err()
}

// MonthFlow is one month of the income/spend trend. In/Out are decimal
// strings; months with no activity carry "0".
type MonthFlow struct {
	Month string `json:"month"` // YYYY-MM
	In    string `json:"in"`
	Out   string `json:"out"`
}

// MonthlyFlow returns income and spend per month for the `months`-wide window
// ending at (and including) month. Every month in the window is present even
// with zero activity, so charts get a continuous axis. Transfers are excluded
// exactly as in Summary.
func MonthlyFlow(ctx context.Context, s *store.Store, view, month string, months int) ([]MonthFlow, error) {
	own, args := ownerFilter(view, 3)
	q := `
		WITH months AS (
		  SELECT generate_series(
		    $1::date - make_interval(months => $2 - 1),
		    $1::date, interval '1 month')::date AS m
		),
		flows AS (
		  SELECT date_trunc('month', t.posted)::date AS m,
		         SUM(t.amount) FILTER (WHERE t.amount > 0) AS inflow,
		         -SUM(t.amount) FILTER (WHERE t.amount < 0) AS outflow
		  FROM transactions t
		  JOIN accounts a ON a.id = t.account_id
		  LEFT JOIN categories c ON c.id = t.category_id
		  WHERE t.posted >= $1::date - make_interval(months => $2 - 1)
		    AND t.posted < ($1::date + interval '1 month')` +
		notTransfer + own + `
		  GROUP BY 1
		)
		SELECT to_char(months.m, 'YYYY-MM'),
		       COALESCE(flows.inflow, 0)::text,
		       COALESCE(flows.outflow, 0)::text
		FROM months
		LEFT JOIN flows ON flows.m = months.m
		ORDER BY months.m`
	full := append([]any{month + "-01", months}, args...)
	rows, err := s.Pool.Query(ctx, q, full...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MonthFlow, 0)
	for rows.Next() {
		var mf MonthFlow
		if err := rows.Scan(&mf.Month, &mf.In, &mf.Out); err != nil {
			return nil, err
		}
		out = append(out, mf)
	}
	return out, rows.Err()
}
