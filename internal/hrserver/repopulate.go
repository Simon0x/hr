package hrserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/budget"
	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/dispatch"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/policy"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
	"github.com/Simon0x/hr/internal/workflow"
)

var goalRe = regexp.MustCompile(`goal ([a-f0-9]{12})`)

func Repopulate(ctx context.Context, root string, pool *pgxpool.Pool, st persistence.Store, reg *contracts.Registry, actor string) (int, error) {
	artifacts, err := st.Load(ctx)
	if err != nil {
		return 0, err
	}
	openFindings, err := workflow.OpenRegisterFindings(root)
	if err != nil {
		return 0, err
	}
	plan, err := workflow.DeriveFrom(artifacts, openFindings)
	if err != nil {
		return 0, err
	}

	seats, err := dispatch.LoadSeats(root)
	if err != nil {
		return 0, err
	}

	p, rawPolicy, err := policy.LoadPolicy(filepath.Join(root, "policies", "default.json"))
	if err != nil {
		return 0, err
	}
	sum := sha256.Sum256(rawPolicy)
	policyDigest := hex.EncodeToString(sum[:])[:16]

	entries, err := st.Read(ctx)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, step := range plan.Ready {
		did, err := evaluateStep(ctx, pool, st, reg, p, policyDigest, artifacts, entries, step, actor, seats)
		if err != nil {
			return created, err
		}
		if did {
			created++
		}
	}
	for _, step := range plan.Blocked {
		did, err := surfaceBlockedStep(ctx, pool, st, reg, step, actor, seats)
		if err != nil {
			return created, err
		}
		if did {
			created++
		}
	}
	return created, nil
}

var blockedOptions = []string{
	"scope and run the cheaper experiment this points to",
	"re-send the department a fresh attempt with stronger input",
	"abandon this line",
}

var escalationOptions = []string{
	"authorize and retry",
	"revise the input so it re-enters the queue with a stronger case",
	"abandon this step",
}

func surfaceBlockedStep(ctx context.Context, pool *pgxpool.Pool, st persistence.Store, reg *contracts.Registry, step workflow.Step, actor string, seats map[string]dispatch.Seat) (bool, error) {
	seat, hasSeat := seats[step.Department]
	if !hasSeat {
		return false, nil
	}
	risk := step.Risk
	if risk == "" {
		risk = seat.Risk
	}
	action := fmt.Sprintf("%s: %s", step.Department, step.Because)
	stepKey := dispatch.StepKey(step)
	return FileException(ctx, pool, st, reg, step, stepKey, risk, action, actor, step.Because, step.Blocked, "irreducible-judgment", blockedOptions)
}

func evaluateStep(ctx context.Context, pool *pgxpool.Pool, st persistence.Store, reg *contracts.Registry, p *policy.Policy, policyDigest string, artifacts []store.Artifact, entries []ledger.Entry, step workflow.Step, actor string, seats map[string]dispatch.Seat) (bool, error) {
	seat, hasSeat := seats[step.Department]
	if !hasSeat {
		return false, nil
	}

	stepKey := dispatch.StepKey(step)
	risk := step.Risk
	if risk == "" {
		risk = seat.Risk
	}
	reversibility := step.Reversibility
	if reversibility == "" {
		reversibility = seat.Reversibility
	}
	confidence := step.Confidence
	if confidence == "" {
		confidence = "likely"
	}
	action := fmt.Sprintf("%s: %s", step.Department, step.Because)

	if m := goalRe.FindStringSubmatch(step.Input); m != nil {
		check := budget.CheckFrom(artifacts, entries, m[1], 0)
		if check.Refused {
			return FileException(ctx, pool, st, reg, step, stepKey, risk, action, actor,
				fmt.Sprintf("budget exceeded for goal %s", m[1]),
				fmt.Sprintf("review spend against goal %s's budget before authorizing more work", m[1]),
				"irreducible-judgment", escalationOptions)
		}
	}

	facts := policy.Facts{
		Action: action, Actor: actor, Risk: risk, Reversibility: reversibility,
		Confidence: confidence, Observability: "logs",
	}
	decision := policy.Evaluate(p, policyDigest, facts, "A2")

	switch decision.Verdict {
	case "allow":
		_, inserted, err := pgstore.InsertJob(ctx, pool, stepKey, step.Department, step.Because, step.Input, risk, reversibility, confidence, "")
		if err != nil {
			return false, err
		}
		if inserted {
			if _, err := st.Append(ctx, ledger.Entry{
				Kind: "decision", Actor: actor, Outcome: "ok",
				Detail: fmt.Sprintf("%s: allow", action),
			}); err != nil {
				return false, err
			}
			_ = pgstore.NotifyJobsChanged(ctx, pool)
		}
		return inserted, nil
	case "escalate":
		return FileException(ctx, pool, st, reg, step, stepKey, risk, action, actor,
			strings.Join(decision.Reasons, "; "),
			fmt.Sprintf("review the %s/%s decision for %q before authorizing", risk, reversibility, action),
			"irreducible-judgment", escalationOptions)
	default:
		return false, nil
	}
}

func FileException(ctx context.Context, pool *pgxpool.Pool, st persistence.Store, reg *contracts.Registry, step workflow.Step, stepKey, risk, action, actor, because, recommendation, class string, options []string) (bool, error) {
	// One definition of a step's identity, shared with dispatch: a subject
	// digest computed from a different recipe here could not be matched back
	// to the job it concerns.
	subjectDigest := sha256.Sum256([]byte(stepKey))

	predicate := map[string]any{
		"because":        because,
		"class":          class,
		"options":        options,
		"recommendation": recommendation,
		"consequence":    risk,
		"stepKey":        stepKey,
	}
	predJSON, err := json.Marshal(predicate)
	if err != nil {
		return false, err
	}

	stmt := struct {
		Type          string              `json:"_type"`
		Subject       []statement.Subject `json:"subject"`
		PredicateType string              `json:"predicateType"`
		Predicate     json.RawMessage     `json:"predicate"`
	}{
		Type: statement.StatementType,
		Subject: []statement.Subject{{
			Name:   fmt.Sprintf("%s-exception", step.Department),
			Digest: map[string]string{"sha256": hex.EncodeToString(subjectDigest[:])},
		}},
		PredicateType: "https://hr.dev/exception/v1",
		Predicate:     predJSON,
	}
	raw, err := json.Marshal(stmt)
	if err != nil {
		return false, err
	}

	canonical, err := statement.Canonical(raw)
	if err != nil {
		return false, err
	}
	problems := contracts.ValidateStatement(reg, canonical, stepKey)
	if len(problems) > 0 {
		return false, fmt.Errorf("exception failed validation: %v", problems)
	}

	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])

	artifact := store.Artifact{
		ID: digest[:12], Kind: "exception", PredicateType: "https://hr.dev/exception/v1",
		Subject: stmt.Subject, Predicate: predicate,
	}

	inserted, err := st.Insert(ctx, artifact, canonical, actor)
	if err != nil {
		return false, err
	}
	if !inserted {
		return false, nil
	}

	if _, err := st.Append(ctx, ledger.Entry{
		Kind: "decision", Actor: actor, Outcome: "escalated",
		Detail: fmt.Sprintf("%s: escalate", action),
	}); err != nil {
		return false, err
	}
	if _, err := st.Append(ctx, ledger.Entry{
		Kind: "emitted", Actor: actor, Outcome: "ok",
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{artifact.ID}},
		Detail:    fmt.Sprintf("exception %s", artifact.ID),
	}); err != nil {
		return false, err
	}
	_ = pgstore.NotifyExceptionsChanged(ctx, pool)
	return true, nil
}
