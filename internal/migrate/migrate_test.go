package migrate

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestUpCreatesTables(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL not set (see README dev section)")
	}
	if err := Up(url); err != nil {
		t.Fatalf("Up: %v", err)
	}
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	for _, tbl := range []string{"accounts", "categories", "transactions", "category_rules", "budgets", "sync_runs"} {
		var n int
		if err := conn.QueryRow(context.Background(),
			`SELECT count(*) FROM information_schema.tables WHERE table_name=$1`, tbl).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s missing (n=%d err=%v)", tbl, n, err)
		}
	}
}
