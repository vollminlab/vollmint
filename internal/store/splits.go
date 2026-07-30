package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	ErrSplitTooFew = errors.New("at least 2 split parts required")
	ErrSplitParent = errors.New("cannot split a transfer or pending transaction")
	ErrSplitSign   = errors.New("every split part must be non-zero and match the transaction's sign")
)

// SplitSumError reports a sum mismatch with the exact expected/received totals
// so the API can echo them back to the client.
type SplitSumError struct {
	Expected, Received string
}

func (e *SplitSumError) Error() string {
	return fmt.Sprintf("split amounts must sum to %s (got %s)", e.Expected, e.Received)
}

type SplitRow struct {
	ID         int64  `json:"id"`
	CategoryID int    `json:"category_id"`
	Category   string `json:"category"`
	Amount     string `json:"amount"`
	Note       string `json:"note"`
}

type SplitInput struct {
	CategoryID int    `json:"category_id"`
	Amount     string `json:"amount"`
	Note       string `json:"note"`
}

// ReplaceSplits atomically replaces a transaction's split set. Invariants
// (sum equals parent amount, matching sign, non-zero parts) are checked in
// Postgres numeric — never in Go floats.
func (s *Store) ReplaceSplits(ctx context.Context, txnID int64, parts []SplitInput) error {
	if len(parts) < 2 {
		return ErrSplitTooFew
	}
	for i, p := range parts {
		if !amountRe.MatchString(p.Amount) {
			return fmt.Errorf("split %d: bad amount %q: %w", i, p.Amount, ErrInvalidAmount)
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var pending bool
	var transferPeer *int64
	err = tx.QueryRow(ctx,
		`SELECT pending, transfer_peer_id FROM transactions WHERE id = $1 FOR UPDATE`,
		txnID).Scan(&pending, &transferPeer)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if pending || transferPeer != nil {
		return ErrSplitParent
	}

	if _, err := tx.Exec(ctx, `DELETE FROM transaction_splits WHERE transaction_id = $1`, txnID); err != nil {
		return err
	}

	b := &pgx.Batch{}
	for _, p := range parts {
		b.Queue(`INSERT INTO transaction_splits (transaction_id, category_id, amount, note)
			VALUES ($1, $2, $3, $4)`, txnID, p.CategoryID, p.Amount, p.Note)
	}
	br := tx.SendBatch(ctx, b)
	for range parts {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return err // FK violations (unknown category) surface here as *pgconn.PgError
		}
	}
	if err := br.Close(); err != nil {
		return err
	}

	var expected, received string
	var sumOK, signsOK bool
	err = tx.QueryRow(ctx, `
		SELECT t.amount::text, SUM(sp.amount)::text, t.amount = SUM(sp.amount),
		       bool_and(sign(sp.amount) = sign(t.amount) AND sp.amount <> 0)
		FROM transactions t
		JOIN transaction_splits sp ON sp.transaction_id = t.id
		WHERE t.id = $1
		GROUP BY t.amount`, txnID).Scan(&expected, &received, &sumOK, &signsOK)
	if err != nil {
		return err
	}
	if !signsOK {
		return ErrSplitSign
	}
	if !sumOK {
		return &SplitSumError{Expected: expected, Received: received}
	}
	return tx.Commit(ctx)
}

// DeleteSplits removes all splits for a transaction. Idempotent — deleting a
// transaction with no splits is a no-op.
func (s *Store) DeleteSplits(ctx context.Context, txnID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM transaction_splits WHERE transaction_id = $1`, txnID)
	return err
}

// SplitsByTxnIDs returns splits for a batch of transactions in one query
// (no N+1). Transactions without splits are simply absent from the map.
func (s *Store) SplitsByTxnIDs(ctx context.Context, ids []int64) (map[int64][]SplitRow, error) {
	out := make(map[int64][]SplitRow)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT sp.transaction_id, sp.id, sp.category_id, c.name, sp.amount::text, sp.note
		FROM transaction_splits sp
		JOIN categories c ON c.id = sp.category_id
		WHERE sp.transaction_id = ANY($1)
		ORDER BY sp.id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var txnID int64
		var r SplitRow
		if err := rows.Scan(&txnID, &r.ID, &r.CategoryID, &r.Category, &r.Amount, &r.Note); err != nil {
			return nil, err
		}
		out[txnID] = append(out[txnID], r)
	}
	return out, rows.Err()
}
