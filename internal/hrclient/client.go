package hrclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/store"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &Client{BaseURL: baseURL, Token: token}
}

type Job struct {
	ID             int64     `json:"id"`
	StepKey        string    `json:"stepKey"`
	Department     string    `json:"department"`
	Because        string    `json:"because"`
	Input          string    `json:"input"`
	Risk           string    `json:"risk"`
	Reversibility  string    `json:"reversibility"`
	Confidence     string    `json:"confidence"`
	ClaimedBy      string    `json:"claimedBy"`
	LeaseToken     string    `json:"leaseToken"`
	LeaseExpiresAt time.Time `json:"leaseExpiresAt"`
}

type ClaimRequest struct {
	Departments []string `json:"departments"`
}

type CompleteRequest struct {
	LeaseToken string         `json:"leaseToken"`
	Outcome    string         `json:"outcome"`
	Artifact   store.Artifact `json:"artifact"`
	Canonical  []byte         `json:"canonical"`
	Entry      ledger.Entry   `json:"entry"`
	// Prior records model invocations this job made before its terminal
	// entry - a fan-out's leads. Appended in order ahead of Entry in the
	// same transaction. The server accepts only invocation entries here, so
	// this stays narrower than the terminal entry rather than becoming a
	// general-purpose way to write the ledger.
	Prior []ledger.Entry `json:"prior,omitempty"`
}

type CompleteResponse struct {
	Entry ledger.Entry `json:"entry"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.httpClient().Do(req)
}

func statusError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s: %s", resp.Status, string(b))
}

// Claim asks for work in departments. The claimant is whoever the bearer
// token resolves to server-side; there is no way to claim as someone else.
func (c *Client) Claim(ctx context.Context, departments []string) (*Job, error) {
	body, err := json.Marshal(ClaimRequest{Departments: departments})
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/jobs/claim", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}
	var j Job
	if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
		return nil, err
	}
	return &j, nil
}

func (c *Client) Complete(ctx context.Context, jobID int64, leaseToken string, artifact store.Artifact, canonical []byte, entry ledger.Entry, prior ...ledger.Entry) (ledger.Entry, bool, error) {
	return c.complete(ctx, jobID, CompleteRequest{
		LeaseToken: leaseToken, Outcome: "ok",
		Artifact: artifact, Canonical: canonical, Entry: entry, Prior: prior,
	})
}

func (c *Client) Fail(ctx context.Context, jobID int64, leaseToken string, entry ledger.Entry, prior ...ledger.Entry) (ledger.Entry, bool, error) {
	return c.complete(ctx, jobID, CompleteRequest{
		LeaseToken: leaseToken, Outcome: "failed", Entry: entry, Prior: prior,
	})
}

func (c *Client) complete(ctx context.Context, jobID int64, req CompleteRequest) (ledger.Entry, bool, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return ledger.Entry{}, false, err
	}
	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/v1/jobs/%d/complete", jobID), body)
	if err != nil {
		return ledger.Entry{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return ledger.Entry{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return ledger.Entry{}, false, statusError(resp)
	}
	var out CompleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ledger.Entry{}, false, err
	}
	return out.Entry, true, nil
}
