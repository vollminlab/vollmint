package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vollminlab/vollmint/internal/simplefin"
	"github.com/vollminlab/vollmint/internal/store"
)

type SyncResult struct {
	Upserted, Categorized, Paired, Swept, SplitsDeleted int
}

// Sync runs one SimpleFIN pull: fetch → upsert accounts+txns → rules →
// transfer matching → pending sweep, recording a sync_runs row either way.
// Window = last successful simplefin run − 7 days (self-healing overlap);
// first run backfills 85 days (SimpleFIN caps a request at 90).
// defaultOwner is assigned to accounts on first sight only (spec: the UI owns
// owner assignment afterwards).
func Sync(ctx context.Context, s *store.Store, c *simplefin.Client, defaultOwner string) (*SyncResult, error) {
	start, err := windowStart(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("window start: %w", err)
	}

	var runID int64
	if err := s.Pool.QueryRow(ctx, `INSERT INTO sync_runs (kind, window_start, window_end)
		VALUES ('simplefin', $1, current_date) RETURNING id`, start).Scan(&runID); err != nil {
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

	set, err := c.Accounts(ctx, start, true)
	if err != nil {
		return fail(fmt.Errorf("simplefin fetch: %w", err))
	}

	res := &SyncResult{}
	var accts []store.Account
	var txns []store.Txn
	for _, a := range set.Accounts {
		accts = append(accts, store.Account{
			ID: a.ID, Name: a.Name, Org: a.Org.Name, Currency: a.Currency,
			Owner: defaultOwner, Balance: a.Balance, BalanceDate: a.BalanceTime(),
		})
		for _, tr := range a.Transactions {
			raw, _ := json.Marshal(tr)
			txns = append(txns, store.Txn{
				Source: "simplefin", ExternalID: tr.ID, AccountID: a.ID,
				Posted: tr.PostedTime(), Amount: tr.Amount,
				Description: tr.Description, Payee: tr.Description,
				Pending: tr.Pending, Raw: raw,
			})
		}
	}
	if err := s.UpsertAccounts(ctx, accts); err != nil {
		return fail(err)
	}
	if res.Upserted, err = s.UpsertTransactions(ctx, txns); err != nil {
		return fail(err)
	}
	if res.SplitsDeleted, err = CleanStaleSplits(ctx, s); err != nil {
		return fail(err)
	}
	if res.Categorized, err = ApplyRules(ctx, s); err != nil {
		return fail(err)
	}
	if res.Paired, err = MatchTransfers(ctx, s); err != nil {
		return fail(err)
	}
	if res.Swept, err = SweepStalePending(ctx, s, 14); err != nil {
		return fail(err)
	}

	status := "ok"
	var details []string
	if res.SplitsDeleted > 0 {
		details = append(details, fmt.Sprintf("stale splits deleted: %d", res.SplitsDeleted))
	}
	if len(set.Errors) > 0 {
		// Institution-level warnings (e.g. one bank needs re-auth): the run
		// still succeeded, but surface them.
		status = "partial"
		details = append(details, set.Errors...)
	}
	detail := strings.Join(details, "; ")
	_, err = s.Pool.Exec(ctx, `UPDATE sync_runs
		SET status=$1, finished=now(), rows_upserted=$2, detail=$3 WHERE id=$4`,
		status, res.Upserted, detail, runID)
	if err != nil {
		return fail(fmt.Errorf("record sync result: %w", err))
	}
	return res, nil
}

// windowStart returns (last successful sync − 7d), or −85d on first run.
func windowStart(ctx context.Context, s *store.Store) (time.Time, error) {
	var last *time.Time
	if err := s.Pool.QueryRow(ctx, `SELECT max(started) FROM sync_runs
		WHERE kind='simplefin' AND status IN ('ok','partial')`).Scan(&last); err != nil {
		return time.Time{}, err
	}
	if last == nil {
		return time.Now().UTC().AddDate(0, 0, -85), nil
	}
	return last.UTC().AddDate(0, 0, -7), nil
}

// CleanStaleSplits deletes split sets whose sum no longer equals the parent
// amount — the parent was re-upserted with a changed amount by sync.
func CleanStaleSplits(ctx context.Context, s *store.Store) (int, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM transaction_splits
		WHERE transaction_id IN (
		  SELECT sp.transaction_id
		  FROM transaction_splits sp
		  JOIN transactions t ON t.id = sp.transaction_id
		  GROUP BY sp.transaction_id, t.amount
		  HAVING SUM(sp.amount) <> t.amount)`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// SweepStalePending deletes pending rows untouched for staleDays — their
// posted replacement arrived under a new id via the overlap window. This is
// the single deliberate exception to "ingestion never deletes": pending rows
// are provisional by definition.
func SweepStalePending(ctx context.Context, s *store.Store, staleDays int) (int, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM transactions
		WHERE pending AND transfer_peer_id IS NULL
		  AND updated_at < now() - make_interval(days => $1)`, staleDays)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
