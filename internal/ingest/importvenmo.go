package ingest

import (
	"context"
	"fmt"
	"io"

	"github.com/vollminlab/vollmint/internal/store"
	"github.com/vollminlab/vollmint/internal/venmo"
)

// ImportVenmo parses one Venmo CSV export and upserts its rows, then runs
// rules + transfer matching so freshly imported rows pair with any waiting
// bank-side VENMO debits. The CSV itself is never persisted (spec).
func ImportVenmo(ctx context.Context, s *store.Store, r io.Reader) (*SyncResult, error) {
	var runID int64
	if err := s.Pool.QueryRow(ctx,
		`INSERT INTO sync_runs (kind) VALUES ('venmo_csv') RETURNING id`).Scan(&runID); err != nil {
		return nil, err
	}
	fail := func(err error) (*SyncResult, error) {
		recoveryCtx := context.WithoutCancel(ctx)
		_, uerr := s.Pool.Exec(recoveryCtx, `UPDATE sync_runs SET status='failed', finished=now(), detail=$1 WHERE id=$2`,
			err.Error(), runID)
		if uerr != nil {
			return nil, fmt.Errorf("%w (also failed to record failure: %v)", err, uerr)
		}
		return nil, err
	}

	txns, err := venmo.Parse(r)
	if err != nil {
		return fail(fmt.Errorf("parse venmo csv: %w", err))
	}
	res := &SyncResult{}
	if res.Upserted, err = s.UpsertTransactions(ctx, txns); err != nil {
		return fail(err)
	}
	if res.Categorized, err = ApplyRules(ctx, s); err != nil {
		return fail(err)
	}
	if res.Paired, err = MatchTransfers(ctx, s); err != nil {
		return fail(err)
	}
	if _, err = s.Pool.Exec(ctx, `UPDATE sync_runs
		SET status='ok', finished=now(), rows_upserted=$1 WHERE id=$2`, res.Upserted, runID); err != nil {
		return fail(fmt.Errorf("record import result: %w", err))
	}
	return res, nil
}
