package pgstore

import (
	"context"
	"testing"
)

func TestCreateIdentity_TokenResolvesBackToTheSameIdentity(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	token, err := CreateIdentity(ctx, pool, "spiffe://hr.local/alice", "Alice", []string{"Engineering", "QA"})
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected a non-empty token")
	}

	id, ok, err := IdentityByToken(ctx, pool, token)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected the freshly created token to resolve")
	}
	if id.SpiffeID != "spiffe://hr.local/alice" || id.DisplayName != "Alice" {
		t.Errorf("resolved identity = %+v, want spiffe://hr.local/alice / Alice", id)
	}
	if len(id.Departments) != 2 || id.Departments[0] != "Engineering" || id.Departments[1] != "QA" {
		t.Errorf("departments = %v, want [Engineering QA]", id.Departments)
	}
}

func TestIdentityByToken_UnknownTokenDoesNotResolve(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if _, err := CreateIdentity(ctx, pool, "spiffe://hr.local/bob", "Bob", []string{UnscopedDepartment}); err != nil {
		t.Fatal(err)
	}

	_, ok, err := IdentityByToken(ctx, pool, "hr_this-was-never-issued")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a token nobody issued should not resolve to an identity")
	}

	_, ok, err = IdentityByToken(ctx, pool, "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("an empty token should not resolve to an identity")
	}
}

func TestCreateIdentity_RawTokenIsNeverStored(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	token, err := CreateIdentity(ctx, pool, "spiffe://hr.local/carol", "Carol", []string{UnscopedDepartment})
	if err != nil {
		t.Fatal(err)
	}

	var stored string
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM identities WHERE spiffe_id = $1`, "spiffe://hr.local/carol").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == token {
		t.Fatal("token_hash equals the raw token - the raw token must never be persisted")
	}
	if stored != HashToken(token) {
		t.Errorf("stored token_hash = %q, want HashToken(token) = %q", stored, HashToken(token))
	}
}

func TestHasAnyIdentity(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	any, err := HasAnyIdentity(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if any {
		t.Fatal("expected no identities on a freshly truncated table")
	}

	if _, err := CreateIdentity(ctx, pool, "spiffe://hr.local/dave", "Dave", []string{UnscopedDepartment}); err != nil {
		t.Fatal(err)
	}

	any, err = HasAnyIdentity(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !any {
		t.Fatal("expected HasAnyIdentity to be true after creating one")
	}
}
