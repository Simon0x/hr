package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Simon0x/hr/internal/budget"
	"github.com/Simon0x/hr/internal/emit"
	"github.com/Simon0x/hr/internal/guard"
	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/policy"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/workflow"
)

var goalRe = regexp.MustCompile(`goal ([a-f0-9]{12})`)

func GoalFromInput(input string) string {
	if m := goalRe.FindStringSubmatch(input); m != nil {
		return m[1]
	}
	return ""
}

type Verdict string

const (
	VerdictNoSeat        Verdict = "no_seat"
	VerdictBudgetRefused Verdict = "budget_refused"
	VerdictRefused       Verdict = "refused"
	VerdictEscalated     Verdict = "escalated"
	VerdictDryRun        Verdict = "dry_run"
	VerdictInvoked       Verdict = "invoked"
	VerdictFailed        Verdict = "failed"
	VerdictBlocked       Verdict = "blocked"
)

type StepResult struct {
	Step          workflow.Step
	Risk          string
	Reversibility string
	Confidence    string
	Goal          string
	Seat          Seat
	HasSeat       bool
	Verdict       Verdict
	Decision      *policy.Decision
	ExceptionPath string
	ExceptionErr  error
	AgentOK       bool
	AgentExit     int
	AgentOutput   string
}

func recordLedger(ctx context.Context, st persistence.Store, kind, outcome, detail, actor, goal string, cost *ledger.Cost) {
	recordInvocation(ctx, st, kind, outcome, detail, actor, goal, cost, nil)
}

func recordInvocation(ctx context.Context, st persistence.Store, kind, outcome, detail, actor, goal string, cost *ledger.Cost, req *ledger.Request) {
	e := ledger.Entry{Kind: kind, Actor: actor, Outcome: outcome, Detail: detail, Cost: cost, Request: req}
	if goal != "" {
		e.Goal = goal
	}
	if _, err := st.Append(ctx, e); err != nil {
		fmt.Fprintf(os.Stderr, "ledger append failed (%s %s: %s): %v\n", kind, outcome, detail, err)
	}
}

// One runs the same budget -> authority -> invoke -> record sequence
// scripts/run implemented, shared by `hr run` and the daemon. It never
// prints; callers format StepResult for their own context.
func One(ctx context.Context, st persistence.Store, root string, step workflow.Step, actor string, execute bool, h harness.Harness) (StepResult, error) {
	result := StepResult{Step: step}

	seats, err := LoadSeats(root)
	if err != nil {
		return result, err
	}
	seat, ok := seats[step.Department]
	if !ok {
		result.Verdict = VerdictNoSeat
		return result, nil
	}
	result.Seat, result.HasSeat = seat, true

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
	result.Risk, result.Reversibility, result.Confidence = risk, reversibility, confidence

	result.Goal = GoalFromInput(step.Input)

	if result.Goal != "" {
		check, err := budget.Check(ctx, st, root, result.Goal, fallbackLeadTokenEstimate)
		if err != nil {
			return result, err
		}
		if check.Refused {
			result.Verdict = VerdictBudgetRefused
			recordLedger(ctx, st, "decision", "refused", fmt.Sprintf("%s: budget exceeded", step.Department), actor, result.Goal, nil)
			return result, nil
		}
	}

	p, raw, err := policy.LoadPolicy(filepath.Join(root, "policies", "default.json"))
	if err != nil {
		return result, err
	}
	sum := sha256.Sum256(raw)
	policyDigest := hex.EncodeToString(sum[:])[:16]

	facts := policy.Facts{
		Action: fmt.Sprintf("%s: %s", step.Department, step.Because),
		Actor:  actor, Risk: risk, Reversibility: reversibility,
		Confidence: confidence, Observability: "logs",
	}
	decision := policy.Evaluate(p, policyDigest, facts, "A2")
	result.Decision = &decision

	outcome := "ok"
	if decision.Verdict == "escalate" {
		outcome = "escalated"
	} else if decision.Verdict != "allow" {
		outcome = "refused"
	}
	recordLedger(ctx, st, "decision", outcome, fmt.Sprintf("%s: %s", step.Department, decision.Verdict), actor, result.Goal, nil)

	if decision.Verdict != "allow" {
		if decision.Verdict == "escalate" {
			result.Verdict = VerdictEscalated
			path, ferr := fileException(ctx, st, root, step, decision, risk, reversibility, confidence, actor)
			if ferr != nil {
				result.ExceptionErr = ferr
			} else {
				result.ExceptionPath = path
			}
		} else {
			result.Verdict = VerdictRefused
		}
		return result, nil
	}

	if !execute {
		result.Verdict = VerdictDryRun
		return result, nil
	}

	stepKey := StepKey(step)

	if seat.FanOut != nil && seat.FanOut.Enabled {
		recordLedger(ctx, st, "action", "claimed", stepKey, actor, result.Goal, nil)
		return runDiscovery(ctx, st, root, step, stepKey, actor, result, h, seat)
	}

	prompt := fmt.Sprintf(
		"Read %s in this repo and follow it as the %s seat.\n\nInput: %s\n\n"+
			"Emit the artifact the procedure specifies by piping an in-toto Statement to "+
			"./hr emit. Its shape is in contracts/predicates/. Do not edit source.",
		seat.Procedure, step.Department, step.Input)
	grant := GrantForDepartment(root, step.Department)

	recordInvocation(ctx, st, "action", "claimed", stepKey, actor, result.Goal, nil,
		NewRequest(root, h, prompt, grant, seat.Procedure))

	// The policy engine follows the agent in: every tool call it makes is
	// judged by the same engine that authorised the step, rather than the
	// step approval standing in for all of them.
	guardToken := guard.NewToken()
	ctx = guard.WithContext(ctx, guard.Context{
		Actor: actor, Goal: result.Goal, Department: step.Department,
		Confidence: confidence, Observability: "logs", Requested: "A2",
		Token: guardToken, Root: root,
	})

	invokeResult, err := h.Invoke(ctx, root, prompt, grant)
	foldGuardDenials(ctx, st, root, guardToken, actor, result.Goal)
	if errors.Is(err, harness.ErrGrantUnenforceable) {
		// The harness refused authority it could not hold to. Nothing ran,
		// so this is an infrastructure block, not a finding against the work.
		result.Verdict = VerdictBlocked
		recordLedger(ctx, st, "action", "blocked", fmt.Sprintf("%s %s: %v", stepKey, step.Department, err), actor, result.Goal, nil)
		path, ferr := fileToolBlockedException(ctx, st, root, step, actor, err.Error())
		if ferr != nil {
			result.ExceptionErr = ferr
		} else {
			result.ExceptionPath = path
		}
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.AgentOK, result.AgentExit, result.AgentOutput = invokeResult.OK, invokeResult.ExitCode, invokeResult.Output

	var cost *ledger.Cost
	if invokeResult.CostUSD > 0 || invokeResult.DurationMS > 0 || invokeResult.Tokens > 0 {
		seconds := float64(invokeResult.DurationMS) / 1000
		cost = &ledger.Cost{Currency: "USD", Amount: &invokeResult.CostUSD, Seconds: &seconds, Tokens: &invokeResult.Tokens}
	}

	if !invokeResult.OK && harness.LooksPermissionBlocked(invokeResult.Output) {
		result.Verdict = VerdictBlocked
		recordLedger(ctx, st, "action", "blocked", fmt.Sprintf("%s %s blocked on tool permission", stepKey, step.Department), actor, result.Goal, cost)
		path, ferr := fileToolBlockedException(ctx, st, root, step, actor, invokeResult.Output)
		if ferr != nil {
			result.ExceptionErr = ferr
		} else {
			result.ExceptionPath = path
		}
		return result, nil
	}

	if invokeResult.OK {
		result.Verdict = VerdictInvoked
	} else {
		result.Verdict = VerdictFailed
	}
	recordOutcome := "failed"
	if invokeResult.OK {
		recordOutcome = "ok"
	}
	recordLedger(ctx, st, "action", recordOutcome, fmt.Sprintf("%s %s ran %s", stepKey, step.Department, seat.Procedure), actor, result.Goal, cost)

	return result, nil
}

// StepKey deterministically identifies a step, so a `claimed` ledger entry
// written before a (possibly long-running) invocation can be matched to its
// terminal entry after — the only way to tell "interrupted" apart from
// "never started" on daemon restart.
func StepKey(step workflow.Step) string {
	sum := sha256.Sum256([]byte(step.Department + "|" + step.Because + "|" + step.Input))
	return hex.EncodeToString(sum[:])[:12]
}

func fileException(ctx context.Context, st persistence.Store, root string, step workflow.Step, decision policy.Decision, risk, reversibility, confidence, actor string) (string, error) {
	predicate := map[string]any{
		"because": strings.Join(decision.Reasons, "; "),
		"class":   "irreducible-judgment",
		"options": []string{
			fmt.Sprintf("authorize manually and re-run: hr authority --action %q --risk %s --reversibility %s --confidence %s --observability logs --requested %s, then hr run --execute",
				decision.Action, risk, reversibility, confidence, decision.Dimensions.Autonomy),
			"revise the input (spec, change, or evidence) so it re-enters the queue with a stronger case",
			"abandon this step",
		},
		"recommendation": fmt.Sprintf("review the %s/%s decision for %q before authorizing", risk, reversibility, decision.Action),
		"consequence":    risk,
	}
	return fileExceptionArtifact(ctx, st, root, step, actor, predicate)
}

func fileToolBlockedException(ctx context.Context, st persistence.Store, root string, step workflow.Step, actor, detail string) (string, error) {
	lines := strings.Split(strings.TrimSpace(detail), "\n")
	summary := lines[len(lines)-1]
	predicate := map[string]any{
		"because": fmt.Sprintf("the %s seat could not get the tool access its procedure needs: %s", step.Department, summary),
		"class":   "missing-capability",
		"options": []string{
			"grant the blocked tool for this session, then hr run --execute to retry",
			"run this step interactively, where a permission prompt can be approved",
			"abandon this step",
		},
		"recommendation": fmt.Sprintf("this is an infrastructure block, not evidence that %s's claim fails - re-run once the tool is granted rather than treating this as a finding", step.Department),
		"consequence":    "R0",
	}
	return fileExceptionArtifact(ctx, st, root, step, actor, predicate)
}

func fileExceptionArtifact(ctx context.Context, st persistence.Store, root string, step workflow.Step, actor string, predicate map[string]any) (string, error) {
	// The step key is what a human resolving this exception acts on: it is
	// the handle the jobs table quarantines by, so an exception that does not
	// carry it can be resolved but not acted on. There is one definition of
	// it (StepKey) rather than a hash recipe per filing site, because three
	// recipes that must agree are three that will not.
	stepKey := StepKey(step)
	predicate["stepKey"] = stepKey
	subjectDigest := sha256.Sum256([]byte(stepKey))

	predJSON, err := json.Marshal(predicate)
	if err != nil {
		return "", err
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
		return "", err
	}

	result, problems, err := emit.Emit(ctx, st, root, raw, actor)
	if err != nil {
		return "", err
	}
	if len(problems) > 0 {
		return "", fmt.Errorf("exception failed validation: %v", problems)
	}
	return result.Rel, nil
}

// foldGuardDenials moves an invocation's refused tool calls into the ledger.
// The guard process writes them to a per-invocation file rather than
// appending directly: the chain is serialized in-process, and a fan-out has
// several agents running at once.
func foldGuardDenials(ctx context.Context, st persistence.Store, root, token, actor, goal string) {
	denials, err := guard.TakeDenials(root, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading guard denials: %v\n", err)
		return
	}
	for _, e := range guard.Entries(denials, actor, goal) {
		if _, aerr := st.Append(ctx, e); aerr != nil {
			fmt.Fprintf(os.Stderr, "recording a refused tool call failed: %v\n", aerr)
		}
	}
}
