package hrserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Simon0x/hr/internal/persistence/pgprovider"
	"github.com/Simon0x/hr/internal/pgstore"
)

func TestHealthz_RequiresNoToken(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, _ := testHTTPServer(t, pool, reg)

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 - /healthz must not require auth", resp.StatusCode)
	}
}

func TestV1Routes_RejectMissingToken(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, _ := testHTTPServer(t, pool, reg)

	resp, err := http.Get(baseURL + "/v1/jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a request with no Authorization header", resp.StatusCode)
	}
}

func TestV1Routes_RejectInvalidToken(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, _ := testHTTPServer(t, pool, reg)

	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/jobs", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a token that matches no identity", resp.StatusCode)
	}
}

func TestV1Routes_NoIdentitiesYet_SaysSoInsteadOfAPlain401(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)

	// Deliberately not using testHTTPServer, which provisions a token - this
	// exercises the truly-first-run state where identities is empty.
	srv := &Server{Pool: pool, Store: pgprovider.Postgres{Pool: pool}, Registry: reg, Exceptions: NewBroadcaster(), Jobs: NewBroadcaster(), Root: root}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/v1/jobs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "hr identity create") {
		t.Errorf("body = %q, want it to point at `hr identity create` when no identities exist yet", body)
	}
}

func TestV1Stream_AcceptsTokenAsQueryParam(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, token := testHTTPServer(t, pool, reg)

	// EventSource cannot set an Authorization header - the stream endpoints
	// must accept ?token= instead, which is what this proves.
	req, err := http.NewRequest(http.MethodGet, baseURL+"/v1/jobs/stream?token="+token, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a stream request authenticated via ?token=", resp.StatusCode)
	}
}

func TestRunDepartment_RecordsTheAuthenticatedIdentityAsActor(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	_, baseURL, token := testHTTPServer(t, pool, reg)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/departments/QA/run",
		strings.NewReader(`{"input":"test the change"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}

	entries, err := pgstore.Read(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Actor == "spiffe://hr.local/test-identity" {
			found = true
		}
		if e.Actor == "spiffe://hr.local/qa" {
			t.Errorf("ledger entry actor = %q, want the resolved identity, not the old hardcoded spiffe://hr.local/<department>", e.Actor)
		}
	}
	if !found {
		t.Errorf("no ledger entry recorded the authenticated identity spiffe://hr.local/test-identity as actor: %+v", entries)
	}
}
