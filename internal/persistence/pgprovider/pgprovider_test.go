package pgprovider_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/persistence/pgprovider"
	"github.com/Simon0x/hr/internal/persistence/storetest"
	"github.com/Simon0x/hr/internal/pgstore"
)

// See internal/pgstore/testdb_test.go - the key must differ from every
// production key (727001 ledger, 727002 migrate), and the lock connection
// must not come from the pool under test or pgxpool.Close hangs.
const testDatabaseLockKey = 727003

func TestPostgres(t *testing.T) {
	dsn := os.Getenv("HR_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("HR_TEST_POSTGRES_URL not set")
	}

	storetest.Run(t, func(t *testing.T) persistence.Store {
		ctx := context.Background()
		lock, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lock.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(testDatabaseLockKey)); err != nil {
			_ = lock.Close(ctx)
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = lock.Close(ctx) })

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		if err := pgstore.Migrate(ctx, pool); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"ledger_entries", "artifacts"} {
			if _, err := pool.Exec(ctx, "TRUNCATE "+table); err != nil {
				t.Fatal(err)
			}
		}
		return pgprovider.Postgres{Pool: pool}
	})
}
