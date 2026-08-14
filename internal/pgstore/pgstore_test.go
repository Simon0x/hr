package pgstore

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
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
	lockTestDatabase(t, dsn)

	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"ledger_entries", "artifacts", "memories", "jobs", "workers", "identities"} {
		if _, err := pool.Exec(ctx, "TRUNCATE "+table); err != nil {
			t.Fatalf("truncating %s: %v", table, err)
		}
	}
	return pool
}

func TestAppend_BuildsValidChain(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		e, err := Append(ctx, pool, ledger.Entry{
			Kind: "action", Actor: "spiffe://hr.local/test",
			Outcome: "ok", Detail: fmt.Sprintf("entry-%d", i),
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if e.Seq != i {
			t.Fatalf("entry %d: seq = %d, want %d", i, e.Seq, i)
		}
	}

	entries, err := Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("read back %d entries, want 5", len(entries))
	}
	if msg, ok := ledger.VerifyChain(entries); !ok {
		t.Fatalf("chain invalid: %s", msg)
	}
}

func TestAppend_PreservesArtifactsAndCostNilness(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	tokens, amount := 42.0, 0.0042
	written, err := Append(ctx, pool, ledger.Entry{
		Kind: "emitted", Actor: "spiffe://hr.local/test",
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{"abc123def456"}},
		Cost:      &ledger.Cost{Tokens: &tokens, Currency: "USD", Amount: &amount, Model: "test-model"},
		Outcome:   "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Append(ctx, pool, ledger.Entry{
		Kind: "decision", Actor: "spiffe://hr.local/test", Outcome: "refused",
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	withArtifacts, withoutArtifacts := entries[0], entries[1]
	if withArtifacts.Artifacts == nil {
		t.Error("entry 0: Artifacts is nil, want non-nil")
	} else if len(withArtifacts.Artifacts.Out) != 1 || withArtifacts.Artifacts.Out[0] != "abc123def456" {
		t.Errorf("entry 0: Artifacts.Out = %v, want [abc123def456]", withArtifacts.Artifacts.Out)
	}
	if withArtifacts.Cost == nil {
		t.Error("entry 0: Cost is nil, want non-nil")
	} else {
		if withArtifacts.Cost.Tokens == nil || *withArtifacts.Cost.Tokens != 42.0 {
			t.Errorf("entry 0: Cost.Tokens = %v, want 42.0", withArtifacts.Cost.Tokens)
		}
		if withArtifacts.Cost.Currency != "USD" {
			t.Errorf("entry 0: Cost.Currency = %q, want USD", withArtifacts.Cost.Currency)
		}
		if withArtifacts.Cost.Amount == nil || *withArtifacts.Cost.Amount != 0.0042 {
			t.Errorf("entry 0: Cost.Amount = %v, want 0.0042", withArtifacts.Cost.Amount)
		}
	}
	if withoutArtifacts.Artifacts != nil {
		t.Errorf("entry 1: Artifacts = %+v, want nil", withoutArtifacts.Artifacts)
	}
	if withoutArtifacts.Cost != nil {
		t.Errorf("entry 1: Cost = %+v, want nil", withoutArtifacts.Cost)
	}

	wantDigest, err := ledger.Digest(written)
	if err != nil {
		t.Fatal(err)
	}
	gotDigest, err := ledger.Digest(withArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	if wantDigest != gotDigest {
		t.Errorf("digest mismatch after round-trip: wrote %s, read back %s", wantDigest, gotDigest)
	}
}

func TestAppend_ConcurrencySerializes(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := Append(ctx, pool, ledger.Entry{
				Kind: "action", Actor: "spiffe://hr.local/test",
				Outcome: "ok", Detail: fmt.Sprintf("concurrent-%d", i),
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent append failed: %v", err)
		}
	}

	entries, err := Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != n {
		t.Fatalf("read back %d entries, want %d (a lost or duplicated append means the "+
			"advisory lock isn't actually serializing)", len(entries), n)
	}
	if msg, ok := ledger.VerifyChain(entries); !ok {
		t.Fatalf("chain invalid after concurrent appends: %s", msg)
	}
}

func TestInsertArtifact_Idempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	a := store.Artifact{
		ID: "abc123def456", Kind: "goal", PredicateType: "https://hr.dev/goal/v1",
		Subject:   []statement.Subject{{Name: "goal-1", Digest: map[string]string{"sha256": "deadbeef"}}},
		Predicate: map[string]any{"outcome": "ship it"},
	}
	canonical := []byte(`{"_type":"x","subject":[],"predicateType":"x","predicate":{}}`)

	for i := 0; i < 2; i++ {
		inserted, err := InsertArtifact(ctx, pool, a, canonical, "spiffe://hr.local/test")
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
		if inserted != (i == 0) {
			t.Errorf("insert %d: inserted = %v, want %v", i, inserted, i == 0)
		}
	}

	got, err := LoadArtifacts(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d artifacts after inserting the same digest twice, want 1", len(got))
	}
	if got[0].ID != a.ID || got[0].Kind != a.Kind || got[0].PredicateType != a.PredicateType {
		t.Errorf("round-tripped artifact = %+v, want %+v", got[0], a)
	}
	if outcome, _ := got[0].Predicate["outcome"].(string); outcome != "ship it" {
		t.Errorf("predicate.outcome = %q, want %q", outcome, "ship it")
	}
}

func TestLoadMemories_ScopeIsAnySlice(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO memories (digest, claim, confidence, source, owner, learned_at, last_verified, scope)
		VALUES ('mem123', 'the sky is blue', 'observed', 'test', 'test', 'now', 'now', ARRAY['sky','color'])`)
	if err != nil {
		t.Fatal(err)
	}

	got, err := LoadMemories(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d memories, want 1", len(got))
	}

	scope, ok := got[0].Predicate["scope"].([]any)
	if !ok {
		t.Fatalf("Predicate[scope] is %T, want []any - memory.Recall's type assertion will silently "+
			"fail and scope-filtering will break", got[0].Predicate["scope"])
	}
	if len(scope) != 2 || scope[0] != "sky" || scope[1] != "color" {
		t.Errorf("scope = %v, want [sky color]", scope)
	}
}
