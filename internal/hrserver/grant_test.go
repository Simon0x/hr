package hrserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/persistence/pgprovider"
	"github.com/Simon0x/hr/internal/pgstore"
)

// scopedServer provisions an identity granted only the named departments.
func scopedServer(t *testing.T, pool *pgxpool.Pool, reg *contracts.Registry, departments ...string) (baseURL, token string) {
	t.Helper()
	s := &Server{Pool: pool, Store: pgprovider.Postgres{Pool: pool}, Registry: reg, Exceptions: NewBroadcaster(), Jobs: NewBroadcaster(), Root: testRoot(t)}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	token, err := pgstore.CreateIdentity(context.Background(), pool, "spiffe://hr.local/scoped", "scoped", departments)
	if err != nil {
		t.Fatal(err)
	}
	return ts.URL, token
}

func do(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestRunDepartment_RefusesADepartmentTheIdentityIsNotGranted(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	baseURL, token := scopedServer(t, pool, reg, "Engineering")

	resp := do(t, http.MethodPost, baseURL+"/v1/departments/QA/run", token, `{"input":"test the change"}`)
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 403: %s", resp.StatusCode, body)
	}

	jobs, err := pgstore.ListJobs(context.Background(), pool, 50, "spiffe://hr.local/scoped")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Errorf("a refused run queued %d job(s); refusal must happen before the job exists", len(jobs))
	}
}

func TestRunDepartment_AllowsAGrantedDepartment(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	baseURL, token := scopedServer(t, pool, reg, "QA")

	resp := do(t, http.MethodPost, baseURL+"/v1/departments/QA/run", token, `{"input":"test the change"}`)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
}

func TestListDepartments_ShowsOnlyWhatTheIdentityMayRun(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	baseURL, token := scopedServer(t, pool, reg, "QA")

	resp := do(t, http.MethodGet, baseURL+"/v1/departments", token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out listDepartmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Departments) != 1 || out.Departments[0].Name != "QA" {
		t.Errorf("listed %+v, want only QA - the UI must not offer a run that would 403", out.Departments)
	}
}

func TestClaim_RefusesWhenNoRequestedDepartmentIsGranted(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	baseURL, token := scopedServer(t, pool, reg, "QA")

	resp := do(t, http.MethodPost, baseURL+"/v1/jobs/claim", token,
		`{"claimedBy":"spiffe://hr.local/scoped","departments":["Engineering","Release"]}`)
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 403: %s", resp.StatusCode, body)
	}
}

// A worker asks for every department it knows how to run. It must receive
// only work its identity is granted, not an ungranted job that happens to be
// queued first.
func TestClaim_NarrowsToTheGrantInsteadOfHandingOverUngrantedWork(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	baseURL, token := scopedServer(t, pool, reg, "QA")

	ctx := context.Background()
	if _, _, err := pgstore.InsertJob(ctx, pool, "key-eng", "Engineering", "b", "i", "R1", "revert", "likely", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pgstore.InsertJob(ctx, pool, "key-qa", "QA", "b", "i", "R1", "revert", "likely", ""); err != nil {
		t.Fatal(err)
	}

	resp := do(t, http.MethodPost, baseURL+"/v1/jobs/claim", token,
		`{"claimedBy":"spiffe://hr.local/scoped","departments":["Engineering","QA"]}`)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	var job struct {
		Department string `json:"department"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.Department != "QA" {
		t.Errorf("claimed a %s job on a QA-only grant", job.Department)
	}
}

// Attribution comes from the token, not the body. A worker that names
// someone else must not be able to write ledger entries under that name.
func TestClaimAndComplete_AttributeToTheAuthenticatedIdentityNotTheBody(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	baseURL, token := scopedServer(t, pool, reg, pgstore.UnscopedDepartment)

	ctx := context.Background()
	if _, _, err := pgstore.InsertJob(ctx, pool, "key-attr", "QA", "b", "i", "R1", "revert", "likely", ""); err != nil {
		t.Fatal(err)
	}

	// The body claims to be someone else entirely.
	resp := do(t, http.MethodPost, baseURL+"/v1/jobs/claim", token,
		`{"claimedBy":"spiffe://hr.local/somebody-else","departments":["QA"]}`)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	var job struct {
		ID         int64  `json:"id"`
		ClaimedBy  string `json:"claimedBy"`
		LeaseToken string `json:"leaseToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.ClaimedBy != "spiffe://hr.local/scoped" {
		t.Fatalf("job claimed by %q - the body chose the actor", job.ClaimedBy)
	}

	// Same for the entry the completion writes.
	fail := `{"leaseToken":"` + job.LeaseToken + `","outcome":"failed","entry":` +
		`{"kind":"action","actor":"spiffe://hr.local/somebody-else","outcome":"failed","detail":"d"}}`
	resp = do(t, http.MethodPost, fmt.Sprintf("%s/v1/jobs/%d/complete", baseURL, job.ID), token, fail)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("complete status = %d, want 200: %s", resp.StatusCode, body)
	}

	entries, err := pgstore.Read(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Actor == "spiffe://hr.local/somebody-else" {
			t.Fatalf("a forged actor reached the ledger: %+v", e)
		}
	}
	if len(entries) == 0 || entries[len(entries)-1].Actor != "spiffe://hr.local/scoped" {
		t.Errorf("terminal entry actor = %+v, want the authenticated identity", entries)
	}
}
