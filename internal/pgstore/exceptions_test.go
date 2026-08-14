package pgstore

import (
	"context"
	"testing"

	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func insertTestException(t *testing.T, db querier, id, createdBy string) store.Artifact {
	t.Helper()
	a := store.Artifact{
		ID: id, Kind: "exception", PredicateType: "https://hr.dev/exception/v1",
		Subject:   []statement.Subject{{Name: "test-exception", Digest: map[string]string{"sha256": id}}},
		Predicate: map[string]any{"because": "test", "class": "test", "options": []string{"a"}, "recommendation": "test", "consequence": "R0"},
	}
	canonical := []byte(`{"_type":"x","subject":[],"predicateType":"x","predicate":{}}`)
	if _, err := InsertArtifact(context.Background(), db, a, canonical, createdBy); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestOpenExceptions_SystemFiledVisibleToEveryone_IdentityFiledPrivate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	insertTestException(t, pool, "excshared00001", "spiffe://hr.local/hr-server")
	insertTestException(t, pool, "excalice000001", "spiffe://hr.local/alice")

	// alice must be a known identity for her exception to actually count as
	// private - an owner that matches no identity is always shared.
	if _, err := CreateIdentity(ctx, pool, "spiffe://hr.local/alice", "Alice", []string{UnscopedDepartment}); err != nil {
		t.Fatal(err)
	}

	aliceView, err := OpenExceptions(ctx, pool, "spiffe://hr.local/alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceView) != 2 {
		t.Fatalf("alice sees %d exceptions, want 2 (the system one and her own)", len(aliceView))
	}

	bobView, err := OpenExceptions(ctx, pool, "spiffe://hr.local/bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(bobView) != 1 || bobView[0].Digest != "excshared00001" {
		t.Fatalf("bob sees %+v, want only the system-filed exception - alice's must stay private", bobView)
	}
}
