package hrserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/hrclient"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence/pgprovider"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func buildValidArtifact(t *testing.T, id string) (store.Artifact, []byte) {
	t.Helper()
	predicate := map[string]any{
		"because":        "test",
		"options":        []string{"a", "b"},
		"recommendation": "test",
		"consequence":    "R0",
	}
	predJSON, err := json.Marshal(predicate)
	if err != nil {
		t.Fatal(err)
	}
	stmt := struct {
		Type          string              `json:"_type"`
		Subject       []statement.Subject `json:"subject"`
		PredicateType string              `json:"predicateType"`
		Predicate     json.RawMessage     `json:"predicate"`
	}{
		Type:          statement.StatementType,
		Subject:       []statement.Subject{{Name: id, Digest: map[string]string{"sha256": "deadbeef"}}},
		PredicateType: "https://hr.dev/exception/v1",
		Predicate:     predJSON,
	}
	raw, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := statement.Canonical(raw)
	if err != nil {
		t.Fatal(err)
	}
	return store.Artifact{
		ID: id, Kind: "exception", PredicateType: "https://hr.dev/exception/v1",
		Subject: stmt.Subject, Predicate: predicate,
	}, canonical
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("HR_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("HR_TEST_POSTGRES_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	lockTestDatabase(t, dsn)

	if err := pgstore.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"ledger_entries", "artifacts", "memories", "jobs", "workers", "identities"} {
		if _, err := pool.Exec(ctx, "TRUNCATE "+table); err != nil {
			t.Fatalf("truncating %s: %v", table, err)
		}
	}
	return pool
}

func testRoot(t *testing.T) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	root := t.TempDir()

	copyFile(t, filepath.Join(repoRoot, "policies", "default.json"), filepath.Join(root, "policies", "default.json"))
	copyFile(t, filepath.Join(repoRoot, "contracts", "statement.schema.json"), filepath.Join(root, "contracts", "statement.schema.json"))

	predicates, err := filepath.Glob(filepath.Join(repoRoot, "contracts", "predicates", "*.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range predicates {
		copyFile(t, p, filepath.Join(root, "contracts", "predicates", filepath.Base(p)))
	}

	capabilities, err := filepath.Glob(filepath.Join(repoRoot, "capabilities", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range capabilities {
		copyFile(t, c, filepath.Join(root, "capabilities", filepath.Base(c)))
	}
	return root
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found upward)")
		}
		dir = parent
	}
}

func testRegistry(t *testing.T, root string) *contracts.Registry {
	t.Helper()
	reg, err := contracts.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func testServer(t *testing.T, pool *pgxpool.Pool, reg *contracts.Registry) *hrclient.Client {
	t.Helper()
	_, url, token := testHTTPServer(t, pool, reg)
	return hrclient.New(url, token)
}

// testHTTPServer also provisions a real identity in pool and returns its
// bearer token, since every /v1/ route now requires one (see auth.go).
func testHTTPServer(t *testing.T, pool *pgxpool.Pool, reg *contracts.Registry) (srv *httptest.Server, url, token string) {
	t.Helper()
	s := &Server{Pool: pool, Store: pgprovider.Postgres{Pool: pool}, Registry: reg, Exceptions: NewBroadcaster(), Jobs: NewBroadcaster(), Root: testRoot(t)}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	token, err := pgstore.CreateIdentity(context.Background(), pool, "spiffe://hr.local/test-identity", "test identity", []string{pgstore.UnscopedDepartment})
	if err != nil {
		t.Fatal(err)
	}
	return ts, ts.URL, token
}

func TestClaimAndComplete_HappyPath(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	client := testServer(t, pool, reg)

	_, inserted, err := pgstore.InsertJob(ctx, pool, "step-1", "QA", "verify change", "artifact-abc", "R0", "revert", "likely", "")
	if err != nil || !inserted {
		t.Fatalf("insert job: inserted=%v err=%v", inserted, err)
	}

	job, err := client.Claim(ctx, []string{"QA"})
	if err != nil {
		t.Fatal(err)
	}
	if job == nil {
		t.Fatal("expected a job, got none")
	}
	if job.Department != "QA" || job.StepKey != "step-1" {
		t.Errorf("claimed wrong job: %+v", job)
	}

	artifact, canonical := buildValidArtifact(t, "resultabc123")
	entry := ledger.Entry{
		Kind: "action", Actor: job.ClaimedBy, Outcome: "ok",
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{artifact.ID}},
	}

	written, ok, err := client.Complete(ctx, job.ID, job.LeaseToken, artifact, canonical, entry)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected completion to succeed")
	}
	if written.Kind != "action" {
		t.Errorf("written entry kind = %q, want action", written.Kind)
	}

	entries, err := pgstore.Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want 1", len(entries))
	}
	if msg, ok := ledger.VerifyChain(entries); !ok {
		t.Fatalf("chain invalid: %s", msg)
	}
}

func TestComplete_RejectsStaleLease(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	client := testServer(t, pool, reg)

	_, _, err := pgstore.InsertJob(ctx, pool, "step-2", "QA", "verify change", "artifact-abc", "R0", "revert", "likely", "")
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.Claim(ctx, []string{"QA"})
	if err != nil || job == nil {
		t.Fatalf("claim: job=%v err=%v", job, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE jobs SET lease_token = gen_random_uuid() WHERE id = $1`, job.ID); err != nil {
		t.Fatal(err)
	}

	artifact, canonical := buildValidArtifact(t, "x")
	entry := ledger.Entry{Kind: "action", Actor: job.ClaimedBy, Outcome: "ok"}

	_, ok, err := client.Complete(ctx, job.ID, job.LeaseToken, artifact, canonical, entry)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected completion with a stale lease token to be rejected")
	}

	entries, err := pgstore.Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d ledger entries after a rejected completion, want 0", len(entries))
	}
}

func TestClaim_ConcurrentClaimersGetDistinctJobs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	client := testServer(t, pool, nil)

	const n = 20
	for i := 0; i < n; i++ {
		_, inserted, err := pgstore.InsertJob(ctx, pool, "step-concurrent-"+string(rune('a'+i)),
			"QA", "verify change", "artifact-abc", "R0", "revert", "likely", "")
		if err != nil || !inserted {
			t.Fatalf("insert job %d: inserted=%v err=%v", i, inserted, err)
		}
	}

	var mu sync.Mutex
	seen := map[int64]bool{}
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			job, err := client.Claim(ctx, []string{"QA"})
			if err != nil {
				errs <- err
				return
			}
			if job == nil {
				errs <- nil
				return
			}
			mu.Lock()
			if seen[job.ID] {
				errs <- fmt.Errorf("job %d claimed twice", job.ID)
			} else {
				seen[job.ID] = true
				errs <- nil
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != n {
		t.Fatalf("got %d distinct claimed jobs, want %d (a duplicate claim means SKIP LOCKED isn't working)", len(seen), n)
	}
}

func TestComplete_RejectsInvalidArtifact(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	client := testServer(t, pool, reg)

	_, _, err := pgstore.InsertJob(ctx, pool, "step-3", "QA", "verify change", "artifact-abc", "R0", "revert", "likely", "")
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.Claim(ctx, []string{"QA"})
	if err != nil || job == nil {
		t.Fatalf("claim: job=%v err=%v", job, err)
	}

	artifact := store.Artifact{ID: "bad", Kind: "exception", PredicateType: "https://hr.dev/exception/v1"}
	canonical := []byte(`{"_type":"https://in-toto.io/Statement/v1","subject":[],"predicateType":"https://hr.dev/exception/v1","predicate":{}}`)
	entry := ledger.Entry{Kind: "action", Actor: job.ClaimedBy, Outcome: "ok"}

	_, ok, err := client.Complete(ctx, job.ID, job.LeaseToken, artifact, canonical, entry)
	if err == nil {
		t.Fatal("expected an error for an artifact that fails contract validation")
	}
	if ok {
		t.Fatal("expected completion with an invalid artifact to be rejected")
	}

	entries, err := pgstore.Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d ledger entries after a rejected completion, want 0", len(entries))
	}
}

func TestComplete_HandlesFailureOutcome(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	client := testServer(t, pool, reg)

	_, _, err := pgstore.InsertJob(ctx, pool, "step-4", "QA", "verify change", "artifact-abc", "R0", "revert", "likely", "")
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.Claim(ctx, []string{"QA"})
	if err != nil || job == nil {
		t.Fatalf("claim: job=%v err=%v", job, err)
	}

	entry := ledger.Entry{Kind: "action", Actor: job.ClaimedBy, Outcome: "failed", Detail: "claude exited 1"}
	written, ok, err := client.Fail(ctx, job.ID, job.LeaseToken, entry)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected failure report to be accepted")
	}
	if written.Outcome != "failed" {
		t.Errorf("written entry outcome = %q, want failed", written.Outcome)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, job.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Errorf("job status = %q, want failed", status)
	}

	artifacts, err := pgstore.LoadArtifacts(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 0 {
		t.Fatalf("got %d artifacts after a failed job, want 0", len(artifacts))
	}
}

func TestListAndResolveExceptions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, token := testHTTPServer(t, pool, reg)

	authedGet := func(url string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return http.DefaultClient.Do(req)
	}
	authedPost := func(url string, body []byte) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		return http.DefaultClient.Do(req)
	}

	artifact, canonical := buildValidArtifact(t, "exc123456789")
	if _, err := pgstore.InsertArtifact(ctx, pool, artifact, canonical, "spiffe://hr.local/test"); err != nil {
		t.Fatal(err)
	}

	listResp, err := authedGet(baseURL + "/v1/exceptions")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listResp.StatusCode)
	}
	var list listExceptionsResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Exceptions) != 1 || list.Exceptions[0].Digest != artifact.ID {
		t.Fatalf("exceptions = %+v, want one with digest %s", list.Exceptions, artifact.ID)
	}

	body, _ := json.Marshal(resolveExceptionRequest{Option: "a"})
	resolveResp, err := authedPost(baseURL+"/v1/exceptions/"+artifact.ID+"/resolve", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resolveResp.Body.Close()
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", resolveResp.StatusCode)
	}

	listResp2, err := authedGet(baseURL + "/v1/exceptions")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp2.Body.Close()
	var list2 listExceptionsResponse
	if err := json.NewDecoder(listResp2.Body).Decode(&list2); err != nil {
		t.Fatal(err)
	}
	if len(list2.Exceptions) != 0 {
		t.Fatalf("exceptions after resolve = %+v, want none (resolved exceptions must not stay open)", list2.Exceptions)
	}
}

func TestRepopulate_CreatesJobForReadyStep(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	root := testRoot(t)
	reg := testRegistry(t, root)

	goalID := "deadbeefcafe"
	goal := store.Artifact{
		ID: goalID, Kind: "goal", PredicateType: "https://hr.dev/goal/v1",
		Subject:   []statement.Subject{{Name: "goal-1", Digest: map[string]string{"sha256": "aa"}}},
		Predicate: map[string]any{"outcome": goalID},
	}
	if _, err := pgstore.InsertArtifact(ctx, pool, goal, []byte(`{}`), "spiffe://hr.local/test"); err != nil {
		t.Fatal(err)
	}

	created, err := Repopulate(ctx, root, pool, pgprovider.Postgres{Pool: pool}, reg, "spiffe://hr.local/hr-server")
	if err != nil {
		t.Fatal(err)
	}
	if created == 0 {
		t.Fatal("expected repopulate to create at least one job for the goal-with-no-problem step")
	}

	entries, err := pgstore.Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := ledger.VerifyChain(entries); !ok {
		t.Fatalf("chain invalid: %s", msg)
	}

	created2, err := Repopulate(ctx, root, pool, pgprovider.Postgres{Pool: pool}, reg, "spiffe://hr.local/hr-server")
	if err != nil {
		t.Fatal(err)
	}
	if created2 != 0 {
		t.Fatalf("second repopulate created %d more jobs, want 0 (step_key idempotency should have blocked it)", created2)
	}
}
