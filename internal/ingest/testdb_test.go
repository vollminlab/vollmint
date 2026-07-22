package ingest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/vollminlab/vollmint/internal/migrate"
	"github.com/vollminlab/vollmint/internal/store"
)

// testDB returns a Store on TEST_DATABASE_URL with migrations applied and
// mutable tables truncated (seed rows in categories/accounts/rules survive
// via re-migration after TRUNCATE ... CASCADE would be complex; instead we
// truncate only transaction-ish tables and restore the venmo account).
func testDB(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL not set (see README dev section)")
	}
	// Serialize DB-backed tests across packages: `go test ./...` runs package
	// binaries concurrently, and every testDB truncates the same shared
	// database. A session advisory lock held for the test's duration keeps
	// them from interleaving.
	lockConn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		t.Fatalf("lock conn: %v", err)
	}
	if _, err := lockConn.Exec(context.Background(), `SELECT pg_advisory_lock(788401)`); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock(788401)`)
		lockConn.Close(context.Background())
	})
	if err := migrate.Up(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s, err := store.New(context.Background(), url)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(s.Close)
	for _, q := range []string{
		`TRUNCATE transactions, sync_runs, budgets RESTART IDENTITY CASCADE`,
		`DELETE FROM accounts WHERE id <> 'venmo'`,
		`DELETE FROM category_rules WHERE priority <> 1000`, // keep only the seed VENMO rule
	} {
		if _, err := s.Pool.Exec(context.Background(), q); err != nil {
			t.Fatalf("reset (%s): %v", q, err)
		}
	}
	return s
}
