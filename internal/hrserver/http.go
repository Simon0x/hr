package hrserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/exceptions"
	"github.com/Simon0x/hr/internal/hrclient"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/web"
)

const defaultLease = 30 * time.Minute

type Server struct {
	Pool *pgxpool.Pool
	// Store is the durable-state seam. Pool stays for the job, identity and
	// exception domains, which are server-only and have no file counterpart
	// to be swapped for.
	Store      persistence.Store
	Registry   *contracts.Registry
	Exceptions *Broadcaster
	Jobs       *Broadcaster
	Root       string
	StartedAt  time.Time

	httpSrv *http.Server
}

// Shutdown drains in-flight requests and stops accepting new ones, waiting
// up to ctx's deadline. Call it after canceling the context Serve was given,
// so the process only exits once the drain completes rather than dropping
// requests when Serve's goroutine is killed by process exit.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	// Every /v1/ route requires a real identity; /healthz and the SPA itself
	// do not, so the app shell can load before a token has been entered.
	api := http.NewServeMux()
	api.HandleFunc("POST /v1/jobs/claim", s.handleClaim)
	api.HandleFunc("POST /v1/jobs/{id}/complete", s.handleComplete)
	api.HandleFunc("GET /v1/jobs", s.handleListJobs)
	api.HandleFunc("GET /v1/jobs/stream", s.handleJobsStream)
	api.HandleFunc("GET /v1/artifacts/{digest}", s.handleGetArtifact)
	api.HandleFunc("GET /v1/history", s.handleArtifactHistory)
	api.HandleFunc("GET /v1/departments", s.handleListDepartments)
	api.HandleFunc("POST /v1/departments/{name}/run", s.handleRunDepartment)
	api.HandleFunc("GET /v1/exceptions", s.handleListExceptions)
	api.HandleFunc("POST /v1/exceptions/{digest}/resolve", s.handleResolveException)
	api.HandleFunc("GET /v1/exceptions/stream", s.handleExceptionsStream)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.Handle("/v1/", s.withAuth(api))
	mux.Handle("/", spaHandler())
	return withCORS(mux)
}

type healthzResponse struct {
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.Pool.Ping(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, healthzResponse{Status: "ok", StartedAt: s.StartedAt})
}

func spaHandler() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(dist, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleClaim(w http.ResponseWriter, r *http.Request) {
	var req hrclient.ClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Departments) == 0 {
		http.Error(w, "departments are required", http.StatusBadRequest)
		return
	}

	// The claimant is the authenticated identity, never the name the body
	// asked for: an actor a client can choose is an actor a client can forge.
	//
	// A worker asks for every department it knows how to run; it may only
	// claim the ones its identity is granted. Narrowing rather than
	// rejecting lets one worker binary serve identities of different scope,
	// but an identity granted none of what it asked for is an error, not an
	// empty queue that looks like idleness.
	id, _ := identityFromContext(r.Context())
	departments := id.GrantedFrom(req.Departments)
	if len(departments) == 0 {
		http.Error(w, fmt.Sprintf("identity %s is not granted any of the requested departments", id.SpiffeID), http.StatusForbidden)
		return
	}

	job, err := pgstore.ClaimJob(r.Context(), s.Pool, id.SpiffeID, departments, defaultLease)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if job == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = pgstore.NotifyJobsChanged(r.Context(), s.Pool)

	writeJSON(w, http.StatusOK, hrclient.Job{
		ID: job.ID, StepKey: job.StepKey, Department: job.Department, Because: job.Because,
		Input: job.Input, Risk: job.Risk, Reversibility: job.Reversibility, Confidence: job.Confidence,
		ClaimedBy: job.ClaimedBy, LeaseToken: job.LeaseToken, LeaseExpiresAt: job.LeaseExpiresAt,
	})
}

func (s *Server) handleComplete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid job id", http.StatusBadRequest)
		return
	}

	var req hrclient.CompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Outcome != "failed" {
		problems := contracts.ValidateStatement(s.Registry, req.Canonical, "job "+r.PathValue("id"))
		if len(problems) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, problems)
			return
		}
	}

	if problem := checkPriorEntries(req.Prior); problem != "" {
		http.Error(w, problem, http.StatusBadRequest)
		return
	}

	// Every entry this request writes is attributed to the caller the token
	// resolved to, whatever the body claimed.
	identity, _ := identityFromContext(r.Context())
	req.Entry.Actor = identity.SpiffeID
	for i := range req.Prior {
		req.Prior[i].Actor = identity.SpiffeID
	}

	ok, written, err := pgstore.CompleteJob(r.Context(), s.Pool, id, identity.SpiffeID, req.LeaseToken, req.Outcome, req.Artifact, req.Canonical, req.Entry, req.Prior...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "job not claimed by this identity/lease, or already completed", http.StatusConflict)
		return
	}
	_ = pgstore.NotifyJobsChanged(r.Context(), s.Pool)

	writeJSON(w, http.StatusOK, hrclient.CompleteResponse{Entry: written})
}

// maxPriorEntries bounds one job's extra invocation records. A fan-out is
// configured in single digits; the cap only stops a runaway or hostile
// worker from growing an append-only log without bound.
const maxPriorEntries = 64

// checkPriorEntries keeps `prior` to what it exists for: records of model
// invocations the job made. Without this it would be a way to write arbitrary
// entries - including a forged `ok` - into the ledger behind a valid lease.
func checkPriorEntries(prior []ledger.Entry) string {
	if len(prior) > maxPriorEntries {
		return fmt.Sprintf("too many prior entries: %d, limit %d", len(prior), maxPriorEntries)
	}
	for i, e := range prior {
		if e.Kind != "action" {
			return fmt.Sprintf("prior[%d]: kind is %q, only \"action\" is accepted", i, e.Kind)
		}
		if e.Request == nil {
			return fmt.Sprintf("prior[%d]: no request - prior entries exist to record model invocations", i)
		}
		if e.Artifacts != nil {
			return fmt.Sprintf("prior[%d]: prior entries do not carry artifacts; the terminal entry does", i)
		}
	}
	return ""
}

type job struct {
	ID            int64   `json:"id"`
	StepKey       string  `json:"stepKey"`
	Department    string  `json:"department"`
	Because       string  `json:"because"`
	Input         string  `json:"input"`
	Risk          string  `json:"risk"`
	Reversibility string  `json:"reversibility"`
	Confidence    string  `json:"confidence"`
	Status        string  `json:"status"`
	Attempts      int     `json:"attempts"`
	ResultDigest  *string `json:"resultDigest,omitempty"`
	Detail        *string `json:"detail,omitempty"`
	CreatedAt     string  `json:"createdAt"`
	ClaimedAt     *string `json:"claimedAt,omitempty"`
}

type listJobsResponse struct {
	Jobs []job `json:"jobs"`
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	id, _ := identityFromContext(r.Context())
	jobs, err := pgstore.ListJobs(r.Context(), s.Pool, 50, id.SpiffeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]job, 0, len(jobs))
	for _, j := range jobs {
		var claimedAt *string
		if j.ClaimedAt != nil {
			s := j.ClaimedAt.Format(time.RFC3339)
			claimedAt = &s
		}
		out = append(out, job{
			ID: j.ID, StepKey: j.StepKey, Department: j.Department, Because: j.Because, Input: j.Input,
			Risk: j.Risk, Reversibility: j.Reversibility, Confidence: j.Confidence, Status: j.Status,
			Attempts: j.Attempts, ResultDigest: j.ResultDigest, Detail: j.Detail,
			CreatedAt: j.CreatedAt.Format(time.RFC3339), ClaimedAt: claimedAt,
		})
	}
	writeJSON(w, http.StatusOK, listJobsResponse{Jobs: out})
}

type artifact struct {
	ID            string              `json:"id"`
	Kind          string              `json:"kind"`
	PredicateType string              `json:"predicateType"`
	Subject       []statement.Subject `json:"subject"`
	Predicate     map[string]any      `json:"predicate"`
}

func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	digest := r.PathValue("digest")
	a, ok, err := pgstore.GetArtifact(r.Context(), s.Pool, digest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, artifact{
		ID: a.ID, Kind: a.Kind, PredicateType: a.PredicateType, Subject: a.Subject, Predicate: a.Predicate,
	})
}

const defaultHistoryPageSize = 25
const maxHistoryPageSize = 100

type artifactHistoryResponse struct {
	Artifacts  []artifact `json:"artifacts"`
	NextBefore string     `json:"nextBefore,omitempty"`
}

// handleArtifactHistory browses the GOAL->PROBLEM->HYPOTHESIS->SPEC->CHANGE->
// VERDICT->RELEASE chain beyond GET /v1/jobs's last-50 window. Exceptions
// have their own endpoint (GET /v1/exceptions) since they can be private to
// an identity, unlike this shared project state.
func (s *Server) handleArtifactHistory(w http.ResponseWriter, r *http.Request) {
	limit := defaultHistoryPageSize
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxHistoryPageSize {
			limit = n
		}
	}

	kind := r.URL.Query().Get("kind")
	if kind == "exception" {
		http.Error(w, "exceptions have their own endpoint: GET /v1/exceptions", http.StatusBadRequest)
		return
	}

	var before *time.Time
	if v := r.URL.Query().Get("before"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			http.Error(w, "invalid before parameter, want RFC3339", http.StatusBadRequest)
			return
		}
		before = &t
	}

	artifacts, err := pgstore.ListArtifactHistory(r.Context(), s.Pool, limit, before, kind)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out := make([]artifact, 0, len(artifacts))
	for _, a := range artifacts {
		out = append(out, artifact{ID: a.ID, Kind: a.Kind, PredicateType: a.PredicateType, Subject: a.Subject, Predicate: a.Predicate})
	}
	resp := artifactHistoryResponse{Artifacts: out}
	if len(artifacts) == limit {
		resp.NextBefore = artifacts[len(artifacts)-1].CreatedAt.Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, resp)
}

type listExceptionsResponse struct {
	Exceptions []exceptions.Exception `json:"exceptions"`
}

func (s *Server) handleListExceptions(w http.ResponseWriter, r *http.Request) {
	id, _ := identityFromContext(r.Context())
	open, err := pgstore.OpenExceptions(r.Context(), s.Pool, id.SpiffeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if open == nil {
		open = []exceptions.Exception{}
	}
	writeJSON(w, http.StatusOK, listExceptionsResponse{Exceptions: open})
}

type resolveExceptionRequest struct {
	Option string `json:"option"`
}

type resolveExceptionResponse struct {
	Entry ledger.Entry `json:"entry"`
}

func (s *Server) handleResolveException(w http.ResponseWriter, r *http.Request) {
	digest := r.PathValue("digest")

	var req resolveExceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Option == "" {
		http.Error(w, "option is required", http.StatusBadRequest)
		return
	}
	id, _ := identityFromContext(r.Context())

	exc, ok, err := pgstore.ExceptionByDigest(r.Context(), s.Pool, digest, id.SpiffeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "exception not found or already resolved", http.StatusNotFound)
		return
	}

	entry, err := pgstore.ResolveException(r.Context(), s.Pool, exc, id.SpiffeID, req.Option)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	_ = pgstore.NotifyExceptionsChanged(r.Context(), s.Pool)

	writeJSON(w, http.StatusOK, resolveExceptionResponse{Entry: entry})
}

func (s *Server) handleExceptionsStream(w http.ResponseWriter, r *http.Request) {
	streamChanges(w, r, s.Exceptions)
}

func (s *Server) handleJobsStream(w http.ResponseWriter, r *http.Request) {
	streamChanges(w, r, s.Jobs)
}

func streamChanges(w http.ResponseWriter, r *http.Request, b *Broadcaster) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "event: changed\ndata: {}\n\n")
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
