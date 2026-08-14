package pgstore

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrations used to be split on a literal ";" in
// Go, which silently mis-splits a dollar-quoted function/trigger body
// containing its own semicolons. This proves the fix - a real Postgres
// connection is required since the whole point is whether Postgres parses
// this correctly, not whether Go's string splitting does.
func TestMigrateFS_AppliesADollarQuotedFunctionBodyAsOneStatement(t *testing.T) {
	dsn := os.Getenv("HR_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("HR_TEST_POSTGRES_URL not set - run `docker compose up postgres` and set it " +
			"(e.g. postgres://hr:hr@localhost:5432/hr) to exercise internal/pgstore")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	// A function body with semicolons inside $$...$$, followed by a second
	// statement - exactly the shape strings.Split(sql, ";") used to shred.
	fsys := fstest.MapFS{
		"migrations/001_dollar_quoted.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS migrate_test_counter (n INT NOT NULL);

			CREATE OR REPLACE FUNCTION migrate_test_bump() RETURNS TRIGGER AS $$
			BEGIN
				NEW.n := NEW.n + 1;
				RETURN NEW;
			END;
			$$ LANGUAGE plpgsql;

			INSERT INTO migrate_test_counter (n) VALUES (1);
		`)},
	}

	if err := migrateFS(ctx, pool, fsys); err != nil {
		t.Fatalf("migrateFS failed on a dollar-quoted function body: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT n FROM migrate_test_counter`).Scan(&n); err != nil {
		t.Fatalf("querying migrate_test_counter: %v", err)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1 (the INSERT after the $$ body must have run as its own statement)", n)
	}

	// Idempotent: re-running must not re-apply (schema_migrations dedup).
	if err := migrateFS(ctx, pool, fsys); err != nil {
		t.Fatalf("second migrateFS call failed: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT n FROM migrate_test_counter`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("n = %d after a second migrateFS call, want 1 (migration re-applied when it shouldn't have)", n)
	}

	pool.Exec(ctx, `DROP TABLE IF EXISTS migrate_test_counter`)
	pool.Exec(ctx, `DROP FUNCTION IF EXISTS migrate_test_bump()`)
	pool.Exec(ctx, `DELETE FROM schema_migrations WHERE filename = '001_dollar_quoted.sql'`)
}
