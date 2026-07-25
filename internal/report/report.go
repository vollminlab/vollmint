// Package report holds read-only aggregation queries for the vollmint API.
// All money is summed in Postgres numeric and returned as decimal strings;
// no float64 ever touches a monetary value.
package report

import (
	"context"
	"fmt"

	"github.com/vollminlab/vollmint/internal/store"
)

// ownerFilter returns the SQL fragment + arg for a view, or ("", nil) for
// household. The alias for transactions is "t" and accounts is "a".
func ownerFilter(view string) (string, []any) {
	switch view {
	case "scott", "nikki", "joint":
		return " AND COALESCE(t.owner_override, a.owner) = $2", []any{view}
	default:
		return "", nil
	}
}

// notTransfer excludes transfer rows from spend/income math.
const notTransfer = ` AND t.transfer_peer_id IS NULL
	AND (c.kind IS NULL OR c.kind <> 'transfer')`

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
	own, args := ownerFilter(view)
	q := `
		SELECT
		  COALESCE(SUM(t.amount) FILTER (WHERE t.amount > 0), 0)::text,
		  COALESCE(-SUM(t.amount) FILTER (WHERE t.amount < 0), 0)::text,
		  COALESCE(-SUM(t.amount) FILTER (WHERE t.amount < 0 AND c.is_vice), 0)::text
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.posted >= $1::date AND t.posted < ($1::date + interval '1 month')` +
		notTransfer + own
	full := append([]any{month + "-01"}, args...)
	if err := s.Pool.QueryRow(ctx, q, full...).Scan(&res.In, &res.Out, &res.Vices); err != nil {
		return res, fmt.Errorf("summary totals: %w", err)
	}
	// Total budget for the month (view-independent — budgets are household).
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM budgets WHERE month = $1::date`,
		month+"-01").Scan(&res.BudgetTotal); err != nil {
		return res, fmt.Errorf("summary budget: %w", err)
	}
	return res, nil
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
	own, args := ownerFilter(view)
	q := `
		SELECT c.id, c.name, (-SUM(t.amount))::text, c.is_vice,
		       COALESCE(b.amount::text, '')
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		JOIN categories c ON c.id = t.category_id
		LEFT JOIN budgets b ON b.category_id = c.id AND b.month = $1::date
		WHERE t.amount < 0
		  AND t.posted >= $1::date AND t.posted < ($1::date + interval '1 month')
		  AND t.transfer_peer_id IS NULL AND c.kind <> 'transfer'` + own + `
		GROUP BY c.id, c.name, c.is_vice, b.amount
		ORDER BY (-SUM(t.amount)) DESC, c.name`
	full := append([]any{month + "-01"}, args...)
	rows, err := s.Pool.Query(ctx, q, full...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategorySpend
	for rows.Next() {
		var cs CategorySpend
		if err := rows.Scan(&cs.CategoryID, &cs.Category, &cs.Spent, &cs.IsVice, &cs.Budget); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}
