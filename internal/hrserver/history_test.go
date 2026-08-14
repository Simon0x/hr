package hrserver

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Simon0x/hr/internal/pgstore"
)

func TestArtifactHistory_HTTP_ListsNewestFirstAndExcludesExceptions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, token := testHTTPServer(t, pool, reg)

	goal, canonical := buildValidArtifact(t, "histgoal000001")
	goal.Kind, goal.PredicateType = "goal", "https://hr.dev/goal/v1"
	if _, err := pgstore.InsertArtifact(ctx, pool, goal, canonical, "spiffe://hr.local/test"); err != nil {
		t.Fatal(err)
	}
	exc, excCanonical := buildValidArtifact(t, "histexc0000001")
	if _, err := pgstore.InsertArtifact(ctx, pool, exc, excCanonical, "spiffe://hr.local/test"); err != nil {
		t.Fatal(err)
	}

	resp := authedGet(t, baseURL+"/v1/history", token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out artifactHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Artifacts) != 1 || out.Artifacts[0].ID != "histgoal000001" {
		t.Fatalf("history = %+v, want only histgoal000001 (the exception artifact must be excluded)", out.Artifacts)
	}
}

func TestArtifactHistory_HTTP_RejectsExceptionKindFilter(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, token := testHTTPServer(t, pool, reg)

	resp := authedGet(t, baseURL+"/v1/history?kind=exception", token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 - use GET /v1/exceptions instead", resp.StatusCode)
	}
}

func TestArtifactHistory_HTTP_RejectsMalformedBeforeCursor(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, token := testHTTPServer(t, pool, reg)

	resp := authedGet(t, baseURL+"/v1/history?before=not-a-timestamp", token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a malformed before cursor", resp.StatusCode)
	}
}
