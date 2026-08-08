package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

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

// ErrNotManual marks balance edits aimed at synced accounts — the sync owns
// those balances and would silently overwrite the edit.
var ErrNotManual = errors.New("account is not manual")

// ErrConflict marks a manual-account id collision.
var ErrConflict = errors.New("already exists")

var manualSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// ManualAccountID derives the deterministic id for a manual account name:
// lowercase, runs of non-alphanumerics collapsed to '-', edges trimmed.
// Returns "manual-" (invalid) when the name has no letters or digits.
func ManualAccountID(name string) string {
	slug := manualSlugRe.ReplaceAllString(strings.ToLower(name), "-")
	return "manual-" + strings.Trim(slug, "-")
}

// CreateManualAccount inserts a manual account and its first snapshot in one
// transaction and returns the derived id. Negative balances are liabilities.
func (s *Store) CreateManualAccount(ctx context.Context, name, owner, balance string) (string, error) {
	if !amountRe.MatchString(balance) {
		return "", ErrInvalidAmount
	}
	id := ManualAccountID(name)
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		INSERT INTO accounts (id, name, org, currency, owner, balance, balance_date, is_manual)
		VALUES ($1, $2, 'Manual', 'USD', $3, $4, CURRENT_DATE, true)
		ON CONFLICT (id) DO NOTHING`, id, name, owner, balance)
	if err != nil {
		return "", fmt.Errorf("insert manual account %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return "", ErrConflict
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO account_balance_snapshots (account_id, snapshot_date, balance)
		VALUES ($1, CURRENT_DATE, $2)`, id, balance); err != nil {
		return "", fmt.Errorf("first snapshot %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit tx: %w", err)
	}
	return id, nil
}

// UpdateManualBalance sets a manual account's balance as of today and upserts
// today's snapshot in the same transaction, so the graph reflects the edit
// immediately. Synced accounts are rejected with ErrNotManual.
func (s *Store) UpdateManualBalance(ctx context.Context, id, balance string) error {
	if !amountRe.MatchString(balance) {
		return ErrInvalidAmount
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	var isManual bool
	err = tx.QueryRow(ctx, `SELECT is_manual FROM accounts WHERE id = $1`, id).Scan(&isManual)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lookup account %s: %w", id, err)
	}
	if !isManual {
		return ErrNotManual
	}
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = $2, balance_date = CURRENT_DATE WHERE id = $1`,
		id, balance); err != nil {
		return fmt.Errorf("update balance %s: %w", id, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO account_balance_snapshots (account_id, snapshot_date, balance)
		VALUES ($1, CURRENT_DATE, $2)
		ON CONFLICT (account_id, snapshot_date) DO UPDATE SET balance = EXCLUDED.balance`,
		id, balance); err != nil {
		return fmt.Errorf("upsert snapshot %s: %w", id, err)
	}
	return tx.Commit(ctx)
}
