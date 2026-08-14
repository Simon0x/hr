package hrserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Simon0x/hr/internal/dispatch"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/policy"
	"github.com/Simon0x/hr/internal/workflow"
)

type department struct {
	Name          string `json:"name"`
	Procedure     string `json:"procedure"`
	Risk          string `json:"risk"`
	Reversibility string `json:"reversibility"`
}

type listDepartmentsResponse struct {
	Departments []department `json:"departments"`
}

func (s *Server) handleListDepartments(w http.ResponseWriter, r *http.Request) {
	seats, err := dispatch.LoadSeats(s.Root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Listing is scoped to what the caller may act on, so the UI never
	// offers a department whose run would come back 403.
	id, _ := identityFromContext(r.Context())
	names := make([]string, 0, len(seats))
	for name := range seats {
		if id.MayAct(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	out := make([]department, 0, len(names))
	for _, name := range names {
		seat := seats[name]
		out = append(out, department{Name: name, Procedure: seat.Procedure, Risk: seat.Risk, Reversibility: seat.Reversibility})
	}
	writeJSON(w, http.StatusOK, listDepartmentsResponse{Departments: out})
}

type runDepartmentRequest struct {
	Input string `json:"input"`
}

type runDepartmentResponse struct {
	Status  string   `json:"status"`
	JobID   int64    `json:"jobId,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

func (s *Server) handleRunDepartment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	seats, err := dispatch.LoadSeats(s.Root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	seat, ok := seats[name]
	if !ok {
		http.Error(w, "unknown department", http.StatusNotFound)
		return
	}

	var req runDepartmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Input == "" {
		http.Error(w, "input is required", http.StatusBadRequest)
		return
	}
	id, _ := identityFromContext(r.Context())
	if !id.MayAct(name) {
		http.Error(w, fmt.Sprintf("identity %s is not granted the %s department", id.SpiffeID, name), http.StatusForbidden)
		return
	}
	actor := id.SpiffeID

	step := workflow.Step{
		Department:    name,
		Because:       "manually run from the web UI",
		Input:         req.Input,
		Risk:          seat.Risk,
		Reversibility: seat.Reversibility,
		Confidence:    "likely",
	}
	stepKey := dispatch.StepKey(step)
	action := fmt.Sprintf("%s: %s", name, step.Because)

	p, rawPolicy, err := policy.LoadPolicy(filepath.Join(s.Root, "policies", "default.json"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(rawPolicy)
	policyDigest := hex.EncodeToString(sum[:])[:16]

	facts := policy.Facts{
		Action: action, Actor: actor, Risk: step.Risk, Reversibility: step.Reversibility,
		Confidence: step.Confidence, Observability: "logs",
	}
	decision := policy.Evaluate(p, policyDigest, facts, "A2")
	ctx := r.Context()

	switch decision.Verdict {
	case "allow":
		job, inserted, err := pgstore.InsertJob(ctx, s.Pool, stepKey, name, step.Because, step.Input, step.Risk, step.Reversibility, step.Confidence, actor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !inserted {
			writeJSON(w, http.StatusOK, runDepartmentResponse{Status: "already_queued"})
			return
		}
		if _, err := s.Store.Append(ctx, ledger.Entry{
			Kind: "decision", Actor: actor, Outcome: "ok",
			Detail: fmt.Sprintf("%s: allow", action),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = pgstore.NotifyJobsChanged(ctx, s.Pool)
		writeJSON(w, http.StatusOK, runDepartmentResponse{Status: "queued", JobID: job.ID})
	case "escalate":
		inserted, err := FileException(ctx, s.Pool, s.Store, s.Registry, step, stepKey, step.Risk, action, actor,
			strings.Join(decision.Reasons, "; "),
			fmt.Sprintf("review the %s/%s decision for %q before authorizing", step.Risk, step.Reversibility, action),
			"irreducible-judgment", escalationOptions)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !inserted {
			writeJSON(w, http.StatusOK, runDepartmentResponse{Status: "already_queued"})
			return
		}
		writeJSON(w, http.StatusOK, runDepartmentResponse{Status: "escalated", Reasons: decision.Reasons})
	default:
		writeJSON(w, http.StatusOK, runDepartmentResponse{Status: "refused", Reasons: decision.Reasons})
	}
}
