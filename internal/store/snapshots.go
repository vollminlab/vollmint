package store

import "context"

// CaptureBalanceSnapshots upserts one balance snapshot per account, keyed on
// the account's balance_date rather than today — a stale feed keeps
// overwriting its last real date instead of fabricating fresh history.
// Idempotent: the evening sync overwrites the morning row for the same date.
// Accounts with no balance or no balance_date (e.g. Venmo) are skipped.
func (s *Store) CaptureBalanceSnapshots(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO account_balance_snapshots (account_id, snapshot_date, balance)
		SELECT id, balance_date, balance
		FROM accounts
		WHERE balance IS NOT NULL AND balance_date IS NOT NULL
		ON CONFLICT (account_id, snapshot_date)
			DO UPDATE SET balance = EXCLUDED.balance`)
	return err
}
