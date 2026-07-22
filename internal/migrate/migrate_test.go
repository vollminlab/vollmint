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
	var cats int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM categories`).Scan(&cats); err != nil || cats < 10 {
		t.Errorf("expected seeded categories, got %d (err=%v)", cats, err)
	}
	var venmoOwner string
	if err := conn.QueryRow(context.Background(), `SELECT owner FROM accounts WHERE id='venmo'`).Scan(&venmoOwner); err != nil || venmoOwner != "scott" {
		t.Errorf("venmo account not seeded (owner=%q err=%v)", venmoOwner, err)
	}
	var ruleCat string
	if err := conn.QueryRow(context.Background(),
		`SELECT c.name FROM category_rules r JOIN categories c ON c.id = r.category_id
		 WHERE r.priority = 1000 AND r.pattern = 'VENMO'`).Scan(&ruleCat); err != nil || ruleCat != "Needs Venmo detail" {
		t.Errorf("VENMO fallback rule not seeded (category=%q err=%v)", ruleCat, err)
	}
}
