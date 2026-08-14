package pgstore

import (
	"context"
	"testing"
	"time"

	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func insertTestArtifact(t *testing.T, db querier, id, kind string) {
	t.Helper()
	a := store.Artifact{
		ID: id, Kind: kind, PredicateType: "https://hr.dev/" + kind + "/v1",
		Subject:   []statement.Subject{{Name: id, Digest: map[string]string{"sha256": id}}},
		Predicate: map[string]any{"outcome": id},
	}
	canonical := []byte(`{"_type":"x","subject":[],"predicateType":"x","predicate":{}}`)
	if _, err := InsertArtifact(context.Background(), db, a, canonical, "spiffe://hr.local/test"); err != nil {
		t.Fatal(err)
	}
}

func TestListArtifactHistory_PaginatesNewestFirst(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	for _, id := range []string{"art1oldest001", "art2middle001", "art3newest001"} {
		insertTestArtifact(t, pool, id, "goal")
		time.Sleep(5 * time.Millisecond) // guarantee distinct created_at ordering
	}

	page1, err := ListArtifactHistory(ctx, pool, 2, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || page1[0].ID != "art3newest001" || page1[1].ID != "art2middle001" {
		t.Fatalf("page1 = %+v, want [art3newest001 art2middle001]", ids(page1))
	}

	page2, err := ListArtifactHistory(ctx, pool, 2, &page1[1].CreatedAt, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].ID != "art1oldest001" {
		t.Fatalf("page2 = %+v, want [art1oldest001]", ids(page2))
	}
}

func TestListArtifactHistory_FiltersByKind(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	insertTestArtifact(t, pool, "goalfiltertest", "goal")
	insertTestArtifact(t, pool, "probfiltertest", "problem")

	goals, err := ListArtifactHistory(ctx, pool, 50, nil, "goal")
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 || goals[0].ID != "goalfiltertest" {
		t.Fatalf("kind=goal results = %+v, want only goalfiltertest", ids(goals))
	}
}

func TestListArtifactHistory_ExcludesExceptions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	insertTestArtifact(t, pool, "goalnotexc0001", "goal")
	insertTestException(t, pool, "excexcluded001", "spiffe://hr.local/hr-server")

	all, err := ListArtifactHistory(ctx, pool, 50, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].ID != "goalnotexc0001" {
		t.Fatalf("unfiltered history = %+v, want only the goal - exceptions must be excluded", ids(all))
	}
}

func ids(artifacts []store.Artifact) []string {
	out := make([]string, len(artifacts))
	for i, a := range artifacts {
		out[i] = a.ID
	}
	return out
}
