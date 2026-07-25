// Package store is the single Postgres access layer for vollmint.
// Amounts are decimal strings end to end — never float64.
package store

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var amountRe = regexp.MustCompile(`^-?\d+(\.\d{1,2})?$`)

type Store struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { s.Pool.Close() }

type Account struct {
	ID, Name, Org, Currency, Owner string
	Balance                        string // decimal string; "" = unknown
	BalanceDate                    time.Time
}

type Txn struct {
	ID          int64
	Source      string // simplefin | venmo_csv
	ExternalID  string
	AccountID   string
	Posted      time.Time
	Amount      string // decimal string, negative = outflow
	Description string
	Payee       string
	Pending     bool
	Raw         []byte // json
}

// UpsertAccounts inserts or updates by id. owner is set only on insert —
// the user may reassign owners in the UI and syncs must not clobber that.
// Validates all accounts before executing any SQL (fail-atomic).
func (s *Store) UpsertAccounts(ctx context.Context, accts []Account) error {
	// Pre-pass: validate all accounts and normalize values
	type normAcct struct {
		id, name, org, currency, owner string
		balance, balanceDate           any
	}
	normalized := make([]normAcct, len(accts))
	for i, a := range accts {
		currency := a.Currency
		if currency == "" {
			currency = "USD"
		}
		var bal any
		if a.Balance != "" {
			if !amountRe.MatchString(a.Balance) {
				return fmt.Errorf("account %s: bad balance %q", a.ID, a.Balance)
			}
			bal = a.Balance
		}
		normalized[i] = normAcct{
			id:          a.ID,
			name:        a.Name,
			org:         a.Org,
			currency:    currency,
			owner:       a.Owner,
			balance:     bal,
			balanceDate: nullTime(a.BalanceDate),
		}
	}

	// Execute all upserts in a transaction
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, n := range normalized {
		_, err := tx.Exec(ctx, `
			INSERT INTO accounts (id, name, org, currency, owner, balance, balance_date)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET
			  name=EXCLUDED.name, org=EXCLUDED.org, currency=EXCLUDED.currency,
			  balance=COALESCE(EXCLUDED.balance, accounts.balance),
			  balance_date=COALESCE(EXCLUDED.balance_date, accounts.balance_date)`,
			n.id, n.name, n.org, n.currency, n.owner, n.balance, n.balanceDate)
		if err != nil {
			return fmt.Errorf("upsert account %s: %w", n.id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// UpsertTransactions inserts or updates by (source, external_id) and returns
// the number of rows written. It never deletes and never touches category_id,
// owner_override, or transfer_peer_id on update (user/matcher-owned fields).
func (s *Store) UpsertTransactions(ctx context.Context, txns []Txn) (int, error) {
	batch := &pgx.Batch{}
	for _, t := range txns {
		if !amountRe.MatchString(t.Amount) {
			return 0, fmt.Errorf("txn %s/%s: bad amount %q", t.Source, t.ExternalID, t.Amount)
		}
		raw := t.Raw
		if len(raw) == 0 {
			raw = []byte(`{}`)
		}
		batch.Queue(`
			INSERT INTO transactions (source, external_id, account_id, posted, amount, description, payee, pending, raw)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (source, external_id) DO UPDATE SET
			  posted=EXCLUDED.posted, amount=EXCLUDED.amount, description=EXCLUDED.description,
			  payee=EXCLUDED.payee, pending=EXCLUDED.pending, raw=EXCLUDED.raw, updated_at=now()`,
			t.Source, t.ExternalID, t.AccountID, t.Posted, t.Amount, t.Description, t.Payee, t.Pending, raw)
	}
	br := s.Pool.SendBatch(ctx, batch)
	defer br.Close()
	n := 0
	for range txns {
		if _, err := br.Exec(); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
