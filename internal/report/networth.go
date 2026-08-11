package report

import (
	"context"
	"fmt"

	"github.com/vollminlab/vollmint/internal/store"
)

// NetWorthPoint is one day of the net-worth series. Total and per-account
// balances are decimal strings. Accounts maps account id -> carried-forward
// balance; accounts with no snapshot on or before the date are absent.
type NetWorthPoint struct {
	Date     string            `json:"date"` // YYYY-MM-DD
	Total    string            `json:"total"`
	Accounts map[string]string `json:"accounts"`
}

// NetWorthAccount is one row of the current-accounts list on the Net Worth
// page. Balance/BalanceDate are "" when the account has never reported one.
type NetWorthAccount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	IsManual    bool   `json:"is_manual"`
	Balance     string `json:"balance"`
	BalanceDate string `json:"balance_date"` // YYYY-MM-DD
}

// accountOwnerFilter filters accounts by owner for scott/nikki/joint views
// using placeholder $argN; household applies no filter. Unlike ownerFilter it
// matches a.owner directly — accounts have no owner_override column.
func accountOwnerFilter(view string, argN int) (string, []any) {
	switch view {
	case "scott", "nikki", "joint":
		return fmt.Sprintf(" AND a.owner = $%d", argN), []any{view}
	}
	return "", nil
}

// NetWorthSeries returns the daily net-worth series for the last `days` days
// (0 = since the earliest snapshot). Each account's value on a date is its
// last snapshot on or before that date; dates before every account's first
// snapshot yield no point at all — history is never fabricated.
func NetWorthSeries(ctx context.Context, s *store.Store, view string, days int) ([]NetWorthPoint, error) {
	own, args := accountOwnerFilter(view, 2)
	q := `
		WITH bounds AS (
		  SELECT CASE
		    WHEN $1::int > 0 THEN CURRENT_DATE - ($1::int - 1)
		    ELSE COALESCE((SELECT MIN(snapshot_date) FROM account_balance_snapshots), CURRENT_DATE)
		  END AS start
		),
		days AS (
		  SELECT generate_series((SELECT start FROM bounds), CURRENT_DATE, interval '1 day')::date AS d
		),
		acct AS (
		  SELECT a.id FROM accounts a WHERE a.active` + own + `
		),
		pts AS (
		  SELECT days.d, acct.id AS account_id, sn.balance
		  FROM days
		  CROSS JOIN acct
		  JOIN LATERAL (
		    SELECT balance FROM account_balance_snapshots s
		    WHERE s.account_id = acct.id AND s.snapshot_date <= days.d
		    ORDER BY s.snapshot_date DESC LIMIT 1
		  ) sn ON true
		)
		SELECT to_char(pts.d, 'YYYY-MM-DD'), pts.account_id, pts.balance::text,
		       SUM(pts.balance) OVER (PARTITION BY pts.d)::text
		FROM pts
		ORDER BY pts.d, pts.account_id`
	full := append([]any{days}, args...)
	rows, err := s.Pool.Query(ctx, q, full...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NetWorthPoint, 0)
	for rows.Next() {
		var date, id, bal, total string
		if err := rows.Scan(&date, &id, &bal, &total); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Date != date {
			out = append(out, NetWorthPoint{Date: date, Total: total, Accounts: map[string]string{}})
		}
		out[len(out)-1].Accounts[id] = bal
	}
	return out, rows.Err()
}

// NetWorthAccounts lists active accounts for the view with their current
// balances, for the account list under the chart.
func NetWorthAccounts(ctx context.Context, s *store.Store, view string) ([]NetWorthAccount, error) {
	own, args := accountOwnerFilter(view, 1)
	q := `
		SELECT a.id, a.name, a.owner, a.is_manual,
		       COALESCE(a.balance::text, ''),
		       COALESCE(to_char(a.balance_date, 'YYYY-MM-DD'), '')
		FROM accounts a
		WHERE a.active` + own + `
		ORDER BY a.name`
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NetWorthAccount, 0)
	for rows.Next() {
		var a NetWorthAccount
		if err := rows.Scan(&a.ID, &a.Name, &a.Owner, &a.IsManual, &a.Balance, &a.BalanceDate); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
