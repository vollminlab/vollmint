package report

import (
	"context"
	"testing"

	"github.com/vollminlab/vollmint/internal/store"
)

// seedNWAccount inserts a bare active account (no balance, no snapshots).
func seedNWAccount(t *testing.T, s *store.Store, id, owner string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		INSERT INTO accounts (id, name, org, currency, owner)
		VALUES ($1, $1, 't', 'USD', $2)`, id, owner); err != nil {
		t.Fatal(err)
	}
}

// seedNWSnap upserts a snapshot daysAgo days before CURRENT_DATE.
func seedNWSnap(t *testing.T, s *store.Store, id string, daysAgo int, balance string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `
		INSERT INTO account_balance_snapshots (account_id, snapshot_date, balance)
		VALUES ($1, CURRENT_DATE - $2::int, $3::numeric)
		ON CONFLICT (account_id, snapshot_date) DO UPDATE SET balance = EXCLUDED.balance`,
		id, daysAgo, balance); err != nil {
		t.Fatal(err)
	}
}

func TestNetWorthSeriesAllRange(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	seedNWAccount(t, s, "a1", "scott")
	seedNWAccount(t, s, "a2", "nikki")
	seedNWSnap(t, s, "a1", 5, "100.00")
	seedNWSnap(t, s, "a2", 2, "50.00")

	series, err := NetWorthSeries(ctx, s, "household", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Axis starts at the earliest snapshot across all accounts (a1, 5 days
	// ago) through today: 6 points.
	if len(series) != 6 {
		t.Fatalf("series len = %d, want 6: %+v", len(series), series)
	}
	if series[0].Date == "" {
		t.Fatalf("series[0] missing date: %+v", series[0])
	}
	// Before a2's first snapshot, only a1 has ever reported — no fabricated
	// history for a2.
	for i := 0; i < 3; i++ {
		if _, ok := series[i].Accounts["a2"]; ok {
			t.Errorf("series[%d] has a2 before its first snapshot: %+v", i, series[i])
		}
		if series[i].Total != "100.00" {
			t.Errorf("series[%d] total = %s, want 100.00 (a1 only)", i, series[i].Total)
		}
	}
	// From a2's first snapshot date onward (index 3 = 2 days ago) totals are
	// a1+a2, carried forward through today.
	for i := 3; i < len(series); i++ {
		if series[i].Total != "150.00" {
			t.Errorf("series[%d] total = %s, want 150.00 (a1+a2)", i, series[i].Total)
		}
		if got := series[i].Accounts["a1"]; got != "100.00" {
			t.Errorf("series[%d] a1 = %s, want 100.00", i, got)
		}
		if got := series[i].Accounts["a2"]; got != "50.00" {
			t.Errorf("series[%d] a2 = %s, want 50.00", i, got)
		}
	}
}

func TestNetWorthSeriesAllRangeEmpty(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// No snapshots anywhere: the COALESCE(..., CURRENT_DATE) fallback in the
	// bounds CTE must still yield an empty, non-nil series (no accounts ever
	// reported a balance, so no series point is fabricated).
	series, err := NetWorthSeries(ctx, s, "household", 0)
	if err != nil {
		t.Fatal(err)
	}
	if series == nil {
		t.Fatal("series is nil, want empty non-nil slice")
	}
	if len(series) != 0 {
		t.Errorf("series len = %d, want 0: %+v", len(series), series)
	}
}
