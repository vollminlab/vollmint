// Package testutil holds helpers shared by DB-backed package tests. It is
// imported only from _test.go files and never reaches production binaries.
package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// dbLockKey serializes all DB-backed tests across package binaries —
// `go test ./...` runs them concurrently against the same shared database.
const dbLockKey = 788401

// SerializeDB blocks until this test holds the cross-package DB lock, and
// releases it (and the connection) when the test finishes. Register order
// matters: the cleanup is registered immediately after Connect, before the
// lock Exec, so a failed lock acquisition cannot leak the connection.
func SerializeDB(t testing.TB, url string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("testutil lock conn: %v", err)
	}
	t.Cleanup(func() {
		conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, dbLockKey)
		conn.Close(context.Background())
	})
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, dbLockKey); err != nil {
		t.Fatalf("testutil advisory lock: %v", err)
	}
}
