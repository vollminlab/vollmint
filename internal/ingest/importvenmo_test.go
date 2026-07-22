package ingest

import (
	"context"
	"os"
	"testing"
)

func TestImportVenmoFileTwiceIsIdempotent(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	f, err := os.Open("../venmo/testdata/venmo-2026.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	res, err := ImportVenmo(ctx, s, f)
	if err != nil {
		t.Fatal(err)
	}
	if res.Upserted != 3 {
		t.Fatalf("first import upserted=%d want 3", res.Upserted)
	}

	f2, _ := os.Open("../venmo/testdata/venmo-2026.csv")
	defer f2.Close()
	res2, err := ImportVenmo(ctx, s, f2)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE source='venmo_csv'`).Scan(&count)
	if count != 3 {
		t.Fatalf("re-import duplicated rows: count=%d (second res=%+v)", count, res2)
	}

	// audit row written
	var kind, status string
	s.Pool.QueryRow(ctx, `SELECT kind, status FROM sync_runs ORDER BY id DESC LIMIT 1`).Scan(&kind, &status)
	if kind != "venmo_csv" || status != "ok" {
		t.Fatalf("sync_runs: kind=%q status=%q", kind, status)
	}
}
