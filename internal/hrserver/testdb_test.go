package hrserver

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// testDatabaseLockKey serializes every test that shares one
// HR_TEST_POSTGRES_URL. Each helper truncates the whole schema on entry, and
// `go test ./...` runs package binaries concurrently, so without this one
// package wipes another's rows mid-test and the failures read as lost
// appends or broken idempotency rather than as interference.
//
// The lock is session-scoped, so it needs a connection held for the whole
// test. That connection is deliberately NOT taken from the pool under test:
// pgxpool.Close blocks until every acquired connection is released, so
// borrowing one here would deadlock teardown.
//
// The key must differ from every production key - 727001 (ledger append)
// and 727002 (migrate) - or holding it here blocks the code under test.
const testDatabaseLockKey = 727003

func lockTestDatabase(t *testing.T, dsn string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(testDatabaseLockKey)); err != nil {
		_ = conn.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
}
