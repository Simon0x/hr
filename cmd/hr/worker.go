package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/dispatch"
	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/hrclient"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
	"github.com/Simon0x/hr/internal/workflow"
)

func workerActor() string {
	if a := os.Getenv("HR_ACTOR"); a != "" {
		return a
	}
	return "spiffe://hr.local/worker"
}

func cmdWorker(root string, args []string) int {
	addr := flagValueOr(args, "server", os.Getenv("HR_SERVER_ADDR"))
	if addr == "" {
		fmt.Fprintln(os.Stderr, "--server or HR_SERVER_ADDR is required")
		return 2
	}
	seats, err := dispatch.LoadSeats(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	departments := flagList(args, "departments")
	if len(departments) == 0 {
		for dept := range seats {
			departments = append(departments, dept)
		}
	}
	poll := 5 * time.Second
	if v, ok := flagValue(args, "poll"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			poll = d
		}
	}

	actor := workerActor()
	client := hrclient.New(addr, os.Getenv("HR_TOKEN"))

	schema, err := os.ReadFile(filepath.Join(root, "contracts", "statement.schema.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("shutting down...")
		cancel()
	}()

	h, err := harness.Select(os.Getenv("HR_HARNESS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	runWorkerLoop(ctx, root, client, reg, schema, actor, departments, addr, poll, seats, h)
	return 0
}

func runWorkerLoop(ctx context.Context, root string, client *hrclient.Client, reg *contracts.Registry, schema []byte, actor string, departments []string, addr string, poll time.Duration, seats map[string]dispatch.Seat, h harness.Harness) {
	fmt.Printf("hr worker — actor %s, departments %v, server %s\n", actor, departments, addr)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := client.Claim(ctx, departments)
		if err != nil {
			fmt.Fprintf(os.Stderr, "claim error: %v\n", err)
			sleepOrDone(ctx, poll)
			continue
		}
		if job == nil {
			sleepOrDone(ctx, poll)
			continue
		}

		fmt.Printf("claimed %s — %s: %s\n", job.StepKey, job.Department, job.Because)
		runJob(ctx, root, client, reg, schema, actor, job, seats, h)
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func runJob(ctx context.Context, root string, client *hrclient.Client, reg *contracts.Registry, schema []byte, actor string, job *hrclient.Job, seats map[string]dispatch.Seat, h harness.Harness) {
	seat, ok := seats[job.Department]
	if !ok {
		failJob(ctx, client, actor, job, fmt.Sprintf("no seat for department %s", job.Department))
		return
	}

	if seat.FanOut != nil && seat.FanOut.Enabled {
		runDiscoveryJob(ctx, root, client, reg, actor, job, seat, h)
		return
	}

	prompt := fmt.Sprintf(
		"Read %s in this repo and follow it as the %s seat.\n\nInput: %s\n\n"+
			"Output the resulting in-toto Statement as your final message, matching the "+
			"provided JSON Schema. Do not run any commands to save it. Do not edit source.",
		seat.Procedure, job.Department, job.Input)
	grant := dispatch.GrantForDepartment(root, job.Department)
	request := dispatch.NewRequest(root, h, prompt, grant, seat.Procedure)

	result, err := h.InvokeStructured(ctx, root, prompt, schema, grant)
	if err != nil {
		failInvocation(ctx, client, actor, job, err.Error(), request)
		return
	}
	if !result.OK && harness.LooksPermissionBlocked(result.Output) {
		failInvocation(ctx, client, actor, job, "BLOCKED, NOT EVIDENCE (needs a human to grant tool access, then retry - this is not a finding against the claim): "+result.Output, request)
		return
	}
	if !result.OK {
		failInvocation(ctx, client, actor, job, fmt.Sprintf("%s exited %d: %s", h.Name(), result.ExitCode, result.Output), request)
		return
	}
	if len(result.StructuredOutput) == 0 {
		failInvocation(ctx, client, actor, job, "no structured output returned", request)
		return
	}

	artifact, canonical, err := parseStructuredArtifact(result.StructuredOutput)
	if err != nil {
		failInvocation(ctx, client, actor, job, "parsing structured output: "+err.Error(), request)
		return
	}
	if problems := contracts.ValidateStatement(reg, canonical, job.StepKey); len(problems) > 0 {
		failInvocation(ctx, client, actor, job, fmt.Sprintf("failed contract validation: %v", problems), request)
		return
	}

	var cost *ledger.Cost
	if result.CostUSD > 0 || result.DurationMS > 0 {
		seconds := float64(result.DurationMS) / 1000
		cost = &ledger.Cost{Currency: "USD", Amount: &result.CostUSD, Seconds: &seconds}
	}

	entry := ledger.Entry{
		Kind: "action", Actor: actor, Outcome: "ok",
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{artifact.ID}},
		Detail:    fmt.Sprintf("%s %s ran %s", job.StepKey, job.Department, seat.Procedure),
		Cost:      cost,
		Request:   request,
	}
	if _, ok, err := client.Complete(ctx, job.ID, job.LeaseToken, artifact, canonical, entry); err != nil {
		fmt.Fprintf(os.Stderr, "complete failed: %v\n", err)
	} else if !ok {
		fmt.Fprintln(os.Stderr, "complete rejected: lease no longer valid")
	}
}

func runDiscoveryJob(ctx context.Context, root string, client *hrclient.Client, reg *contracts.Registry, actor string, job *hrclient.Job, seat dispatch.Seat, h harness.Harness) {
	step := workflow.Step{Department: job.Department, Because: job.Because, Input: job.Input}
	goal := dispatch.GoalFromInput(job.Input)

	outcome, err := dispatch.RunDiscovery(ctx, persistence.File{Root: root}, root, step, goal, h, seat)
	if err != nil {
		failJob(ctx, client, actor, job, err.Error())
		return
	}

	// One entry per lead invocation, sent with whatever terminal entry
	// follows, so the worker path records the same invocations `hr run` does.
	leads := dispatch.LeadEntries(root, h, outcome, actor, goal, seat.Procedure)

	if outcome.BudgetRefused {
		failInvocation(ctx, client, actor, job, fmt.Sprintf("%d-lead fan-out exceeds budget", len(outcome.Leads)), nil, leads...)
		return
	}
	if outcome.AllBlocked {
		detail := "blocked on tool permission"
		if len(outcome.Leads) > 0 {
			detail = outcome.Leads[0].Result.Output
		}
		failInvocation(ctx, client, actor, job, "BLOCKED, NOT EVIDENCE (needs a human to grant tool access, then retry - this is not a finding against the claim): "+detail, nil, leads...)
		return
	}
	if outcome.Settled == 0 {
		failInvocation(ctx, client, actor, job, "every lead failed", nil, leads...)
		return
	}
	if !outcome.Synthesized {
		failInvocation(ctx, client, actor, job, fmt.Sprintf("synthesis produced no artifact: exit %d: %s", outcome.SynthesisExit, outcome.SynthesisOutput), outcome.SynthesisRequest, leads...)
		return
	}

	artifact, canonical, err := parseStructuredArtifact(outcome.Statement)
	if err != nil {
		failInvocation(ctx, client, actor, job, "parsing structured output: "+err.Error(), outcome.SynthesisRequest, leads...)
		return
	}
	if problems := contracts.ValidateStatement(reg, canonical, job.StepKey); len(problems) > 0 {
		failInvocation(ctx, client, actor, job, fmt.Sprintf("failed contract validation: %v", problems), outcome.SynthesisRequest, leads...)
		return
	}

	detail := fmt.Sprintf("%s %s synthesized from %d/%d settled leads", job.StepKey, job.Department, outcome.Settled, len(outcome.Leads))
	if outcome.VerifyErr != nil || !outcome.Verify.OK {
		detail += " (independent validate pass failed to run)"
	}

	var cost *ledger.Cost
	if outcome.TotalCostUSD > 0 || outcome.TotalSeconds > 0 || outcome.TotalTokens > 0 {
		cost = &ledger.Cost{Currency: "USD", Amount: &outcome.TotalCostUSD, Seconds: &outcome.TotalSeconds, Tokens: &outcome.TotalTokens}
	}

	entry := ledger.Entry{
		Kind: "action", Actor: actor, Outcome: "ok",
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{artifact.ID}},
		Detail:    detail,
		Cost:      cost,
		Request:   outcome.SynthesisRequest,
	}
	if _, ok, err := client.Complete(ctx, job.ID, job.LeaseToken, artifact, canonical, entry, leads...); err != nil {
		fmt.Fprintf(os.Stderr, "complete failed: %v\n", err)
	} else if !ok {
		fmt.Fprintln(os.Stderr, "complete rejected: lease no longer valid")
	}
}

func failJob(ctx context.Context, client *hrclient.Client, actor string, job *hrclient.Job, detail string) {
	failInvocation(ctx, client, actor, job, detail, nil)
}

// failInvocation is failJob for a failure that follows a model invocation:
// the entry still records what that model was asked.
func failInvocation(ctx context.Context, client *hrclient.Client, actor string, job *hrclient.Job, detail string, req *ledger.Request, prior ...ledger.Entry) {
	entry := ledger.Entry{Kind: "action", Actor: actor, Outcome: "failed", Detail: detail, Request: req}
	if _, ok, err := client.Fail(ctx, job.ID, job.LeaseToken, entry, prior...); err != nil {
		fmt.Fprintf(os.Stderr, "fail-report failed: %v\n", err)
	} else if !ok {
		fmt.Fprintln(os.Stderr, "fail-report rejected: lease no longer valid")
	}
}

func parseStructuredArtifact(raw json.RawMessage) (store.Artifact, []byte, error) {
	canonical, err := statement.Canonical(raw)
	if err != nil {
		return store.Artifact{}, nil, err
	}
	var stmt statement.Statement
	if err := json.Unmarshal(canonical, &stmt); err != nil {
		return store.Artifact{}, nil, err
	}
	if stmt.PredicateType == "" {
		return store.Artifact{}, nil, fmt.Errorf("missing predicateType")
	}
	var predicate map[string]any
	if err := json.Unmarshal(stmt.Predicate, &predicate); err != nil {
		return store.Artifact{}, nil, err
	}
	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])
	return store.Artifact{
		ID: digest[:12], Kind: store.KindOf(stmt.PredicateType), PredicateType: stmt.PredicateType,
		Subject: stmt.Subject, Predicate: predicate,
	}, canonical, nil
}
