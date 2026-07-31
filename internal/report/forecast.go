package report

import (
	"context"
	"fmt"

	"github.com/vollminlab/vollmint/internal/store"
)

// ForecastBill is one detected recurring bill and its status for the month.
type ForecastBill struct {
	Payee          string `json:"payee"`
	CategoryID     *int   `json:"category_id"`
	Category       string `json:"category"`
	PredictedDay   int    `json:"predicted_day"`
	ExpectedAmount string `json:"expected_amount"`
	Paid           bool   `json:"paid"`
	PaidDate       string `json:"paid_date"`
	PaidAmount     string `json:"paid_amount"`
}

// ForecastResult is the month's recurring-bill forecast.
type ForecastResult struct {
	Month             string         `json:"month"`
	View              string         `json:"view"`
	Bills             []ForecastBill `json:"bills"`
	RemainingExpected string         `json:"remaining_expected"`
}

// Forecast predicts this month's recurring bills from payee history.
// A payee qualifies as a bill when it has spend in >=3 distinct months
// overall AND in >=2 of the 3 months before the target month (recent
// cadence — excludes payees that have gone dead). P2P payees (Venmo/Zelle)
// are excluded — same-payee grouping is meaningless for them; splits are
// the tool there. Each bill's predicted day is the median day-of-month of
// its last 6 months of charges; its expected amount is the most recent
// charge. A bill is "paid" if the payee has a non-pending charge already
// posted in the target month. RemainingExpected sums ExpectedAmount over
// unpaid bills only.
func Forecast(ctx context.Context, s *store.Store, view, month string) (ForecastResult, error) {
	res := ForecastResult{Month: month, View: view,
		Bills: []ForecastBill{}, RemainingExpected: "0"}

	own, args := ownerFilter(view, 2)
	full := append([]any{month + "-01"}, args...)

	rows, err := s.Pool.Query(ctx, `
WITH spend AS (
  SELECT t.id, t.payee, -t.amount AS mag, t.posted, t.pending,
         date_trunc('month', t.posted)::date AS m, t.category_id
  FROM transactions t
  JOIN accounts a ON a.id = t.account_id
  LEFT JOIN categories c ON c.id = t.category_id
  WHERE t.amount < 0 AND t.payee <> ''
    AND t.transfer_peer_id IS NULL
    AND (c.kind IS NULL OR c.kind <> 'transfer')
    AND t.payee NOT ILIKE '%venmo%' AND t.payee NOT ILIKE '%zelle%'`+own+`
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
),
med AS (
  SELECT payee,
         round(percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(day FROM posted)))::int AS pday
  FROM hist
  WHERE posted >= ($1::date - interval '6 months') AND posted < $1::date
  GROUP BY payee
),
latest AS (
  SELECT DISTINCT ON (payee) payee, mag AS expected
  FROM hist ORDER BY payee, posted DESC, id DESC
),
catmode AS (
  SELECT payee, mode() WITHIN GROUP (ORDER BY category_id) AS category_id
  FROM hist WHERE category_id IS NOT NULL GROUP BY payee
),
paid AS (
  SELECT DISTINCT ON (payee) payee, posted, mag
  FROM spend
  WHERE NOT pending AND posted >= $1::date AND posted < ($1::date + interval '1 month')
  ORDER BY payee, posted ASC, id ASC
)
SELECT cad.payee, cm.category_id, COALESCE(c.name, ''),
       COALESCE(md.pday, 1), COALESCE(lt.expected::text, '0'),
       (p.payee IS NOT NULL),
       COALESCE(to_char(p.posted, 'YYYY-MM-DD'), ''), COALESCE(p.mag::text, ''),
       COALESCE(SUM(CASE WHEN p.payee IS NULL THEN lt.expected ELSE 0 END) OVER (), 0)::text
FROM cadence cad
LEFT JOIN med md ON md.payee = cad.payee
LEFT JOIN latest lt ON lt.payee = cad.payee
LEFT JOIN catmode cm ON cm.payee = cad.payee
LEFT JOIN categories c ON c.id = cm.category_id
LEFT JOIN paid p ON p.payee = cad.payee
ORDER BY (p.payee IS NOT NULL),
         CASE WHEN p.payee IS NULL THEN COALESCE(md.pday, 1) END,
         p.posted, cad.payee`, full...)
	if err != nil {
		return res, fmt.Errorf("forecast: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b ForecastBill
		var remaining string
		if err := rows.Scan(&b.Payee, &b.CategoryID, &b.Category,
			&b.PredictedDay, &b.ExpectedAmount, &b.Paid,
			&b.PaidDate, &b.PaidAmount, &remaining); err != nil {
			return res, fmt.Errorf("forecast scan: %w", err)
		}
		res.RemainingExpected = remaining
		res.Bills = append(res.Bills, b)
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("forecast rows: %w", err)
	}
	return res, nil
}
