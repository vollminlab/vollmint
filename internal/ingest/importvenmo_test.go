package ingest

import (
	"context"
	"os"
	"strings"
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
	if res2.Upserted != 3 {
		t.Fatalf("second import upserted=%d want 3", res2.Upserted)
	}
	var count int
	s.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE source='venmo_csv'`).Scan(&count)
	if count != 3 {
		t.Fatalf("re-import duplicated rows: count=%d (second res=%+v)", count, res2)
	}

	// audit row written
	var kind, status string
	var rowsUp int
	s.Pool.QueryRow(ctx, `SELECT kind, status, rows_upserted FROM sync_runs ORDER BY id DESC LIMIT 1`).Scan(&kind, &status, &rowsUp)
	if kind != "venmo_csv" || status != "ok" || rowsUp != 3 {
		t.Fatalf("sync_runs: kind=%q status=%q rows_upserted=%d", kind, status, rowsUp)
	}
}

func TestImportVenmoRecordsFailure(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()
	if _, err := ImportVenmo(ctx, s, strings.NewReader("not,a,venmo,export\n")); err == nil {
		t.Fatal("want error on garbage CSV")
	}
	var kind, status, detail string
	s.Pool.QueryRow(ctx, `SELECT kind, status, coalesce(detail,'') FROM sync_runs
		ORDER BY id DESC LIMIT 1`).Scan(&kind, &status, &detail)
	if kind != "venmo_csv" || status != "failed" || detail == "" {
		t.Fatalf("failure not recorded: kind=%q status=%q detail=%q", kind, status, detail)
	}
}
