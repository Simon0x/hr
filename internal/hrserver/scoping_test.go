package hrserver

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Simon0x/hr/internal/pgstore"
)

func authedGet(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestListJobs_HTTP_OwnedJobsAreNotVisibleToOtherIdentities(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, aliceToken := testHTTPServer(t, pool, reg)

	bobToken, err := pgstore.CreateIdentity(ctx, pool, "spiffe://hr.local/bob", "Bob", []string{pgstore.UnscopedDepartment})
	if err != nil {
		t.Fatal(err)
	}

	if _, inserted, err := pgstore.InsertJob(ctx, pool, "step-shared", "QA", "system work", "in", "R0", "revert", "likely", ""); err != nil || !inserted {
		t.Fatalf("insert shared job: inserted=%v err=%v", inserted, err)
	}
	if _, inserted, err := pgstore.InsertJob(ctx, pool, "step-alice-only", "QA", "alice's own work", "in", "R0", "revert", "likely", "spiffe://hr.local/test-identity"); err != nil || !inserted {
		t.Fatalf("insert alice's job: inserted=%v err=%v", inserted, err)
	}

	resp := authedGet(t, baseURL+"/v1/jobs", aliceToken)
	defer resp.Body.Close()
	var aliceList listJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&aliceList); err != nil {
		t.Fatal(err)
	}
	if len(aliceList.Jobs) != 2 {
		t.Fatalf("alice (the job's owner) sees %d jobs, want 2", len(aliceList.Jobs))
	}

	resp2 := authedGet(t, baseURL+"/v1/jobs", bobToken)
	defer resp2.Body.Close()
	var bobList listJobsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&bobList); err != nil {
		t.Fatal(err)
	}
	if len(bobList.Jobs) != 1 || bobList.Jobs[0].StepKey != "step-shared" {
		t.Fatalf("bob sees %+v, want only the shared job - alice's private job leaked", bobList.Jobs)
	}
}

func TestListExceptions_HTTP_OwnedExceptionsAreNotVisibleToOtherIdentities(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, aliceToken := testHTTPServer(t, pool, reg)

	bobToken, err := pgstore.CreateIdentity(ctx, pool, "spiffe://hr.local/bob", "Bob", []string{pgstore.UnscopedDepartment})
	if err != nil {
		t.Fatal(err)
	}

	shared, _ := buildValidArtifact(t, "sharedexc0001")
	if _, err := pgstore.InsertArtifact(ctx, pool, shared, []byte(`{}`), "spiffe://hr.local/hr-server"); err != nil {
		t.Fatal(err)
	}
	aliceOwned, _ := buildValidArtifact(t, "aliceexc00001")
	if _, err := pgstore.InsertArtifact(ctx, pool, aliceOwned, []byte(`{}`), "spiffe://hr.local/test-identity"); err != nil {
		t.Fatal(err)
	}

	resp := authedGet(t, baseURL+"/v1/exceptions", aliceToken)
	defer resp.Body.Close()
	var aliceList listExceptionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&aliceList); err != nil {
		t.Fatal(err)
	}
	if len(aliceList.Exceptions) != 2 {
		t.Fatalf("alice (the exception's owner) sees %d exceptions, want 2", len(aliceList.Exceptions))
	}

	resp2 := authedGet(t, baseURL+"/v1/exceptions", bobToken)
	defer resp2.Body.Close()
	var bobList listExceptionsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&bobList); err != nil {
		t.Fatal(err)
	}
	if len(bobList.Exceptions) != 1 || bobList.Exceptions[0].Digest != shared.ID {
		t.Fatalf("bob sees %+v, want only the shared exception - alice's private one leaked", bobList.Exceptions)
	}
}
