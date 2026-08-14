package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/workflow"
)

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

	agents, err := filepath.Glob(filepath.Join(repoRoot, "agents", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		copyFile(t, a, filepath.Join(root, "agents", filepath.Base(a)))
	}

	// The real SKILL.md files carry the seats' tool grants; without them a
	// test would exercise the read-only fallback and prove nothing about
	// what a configured seat actually runs under.
	skills, err := filepath.Glob(filepath.Join(repoRoot, "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range skills {
		copyFile(t, s, filepath.Join(root, "skills", filepath.Base(filepath.Dir(s)), "SKILL.md"))
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

func TestOne_AllowInvokesHarness(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{Result: harness.Result{OK: true, ExitCode: 0, Output: "done"}}
	step := workflow.Step{Department: "QA", Because: "verify change", Input: "artifact-abc123def456.json"}

	result, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", true, mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictInvoked {
		t.Fatalf("verdict = %s, want %s (reasons: %v)", result.Verdict, VerdictInvoked, result.Decision.Reasons)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("harness invoked %d times, want 1", len(mock.Calls))
	}
	if mock.Calls[0].Root != root {
		t.Errorf("harness called with root %q, want %q", mock.Calls[0].Root, root)
	}
	if !strings.Contains(mock.Calls[0].Prompt, "departments/qa.md") {
		t.Errorf("prompt %q does not mention the seat's procedure", mock.Calls[0].Prompt)
	}

	entries, err := ledger.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	var sawClaimed, sawOK bool
	for _, e := range entries {
		if e.Kind != "action" {
			continue
		}
		switch e.Outcome {
		case "claimed":
			sawClaimed = true
		case "ok":
			sawOK = true
		}
	}
	if !sawClaimed || !sawOK {
		t.Errorf("expected claimed and ok action entries in the ledger, got %+v", entries)
	}
}

// The invariant: the claimed entry must describe the invocation that
// actually happened - same prompt, same grant, same harness - and must be
// written before it, so an interrupted run still says what was asked.
func TestOne_ClaimedEntryRecordsWhatTheHarnessWasAsked(t *testing.T) {
	root := testRoot(t)
	copyFile(t, filepath.Join(findRepoRoot(t), "departments", "qa.md"), filepath.Join(root, "departments", "qa.md"))
	mock := &harness.Mock{Result: harness.Result{OK: true}}
	step := workflow.Step{Department: "QA", Because: "verify change", Input: "artifact-abc123def456.json"}

	if _, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", true, mock); err != nil {
		t.Fatal(err)
	}

	entries, err := ledger.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	var claimed *ledger.Entry
	for i, e := range entries {
		if e.Kind == "action" && e.Outcome == "claimed" {
			claimed = &entries[i]
		}
	}
	if claimed == nil {
		t.Fatal("no claimed entry")
	}
	if claimed.Request == nil {
		t.Fatal("claimed entry carries no request - the invocation is unauditable")
	}

	call := mock.Calls[0]
	if claimed.Request.PromptDigest != ledger.TextDigest(call.Prompt) {
		t.Error("logged prompt digest does not match the prompt the harness received")
	}
	if strings.Join(claimed.Request.Tools, ",") != strings.Join(call.Grant.Allow, ",") {
		t.Errorf("logged grant %v does not match the grant the harness received %v", claimed.Request.Tools, call.Grant.Allow)
	}
	if claimed.Request.Harness != mock.Name() {
		t.Errorf("logged harness = %q, want %q", claimed.Request.Harness, mock.Name())
	}
	if strings.Join(claimed.Request.DeniedTools, ",") != strings.Join(call.Grant.Deny, ",") {
		t.Errorf("logged denials %v do not match what the harness was given %v", claimed.Request.DeniedTools, call.Grant.Deny)
	}
	if len(call.Grant.Deny) == 0 {
		t.Error("the QA seat declares disallowed-tools but the harness received no denials")
	}
	for _, spec := range call.Grant.Allow {
		if spec == "Bash" {
			t.Errorf("harness received bare Bash for a seat that granted only scoped commands: %v", call.Grant.Allow)
		}
	}
	if claimed.Request.Procedure != "departments/qa.md" {
		t.Errorf("logged procedure = %q, want departments/qa.md", claimed.Request.Procedure)
	}
	raw, err := os.ReadFile(filepath.Join(root, "departments", "qa.md"))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Request.ProcedureDigest != ledger.TextDigest(string(raw)) {
		t.Error("logged procedure digest does not pin the procedure that was on disk")
	}
}

func TestOne_EscalateDoesNotInvokeHarness(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{Result: harness.Result{OK: true}}
	step := workflow.Step{Department: "Release", Because: "release change", Input: "artifact-abc123def456.json"}

	result, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", true, mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictEscalated {
		t.Fatalf("verdict = %s, want %s (reasons: %v)", result.Verdict, VerdictEscalated, result.Decision.Reasons)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("harness invoked %d times, want 0 — an escalated decision must never reach the agent", len(mock.Calls))
	}
	if result.ExceptionErr != nil {
		t.Fatalf("fileException failed: %v", result.ExceptionErr)
	}
	if result.ExceptionPath == "" {
		t.Fatal("expected an exception artifact path")
	}
	if _, err := os.Stat(filepath.Join(root, result.ExceptionPath)); err != nil {
		t.Errorf("exception artifact not found on disk: %v", err)
	}
}

func TestOne_SingleCallBudgetRefusalSkipsHarness(t *testing.T) {
	root := testRoot(t)
	writeGoalArtifact(t, root, "aaaaaaaaaaaa", "1000")
	mock := &harness.Mock{Result: harness.Result{OK: true}}
	step := workflow.Step{Department: "QA", Because: "verify change", Input: "goal aaaaaaaaaaaa — verify change"}

	result, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", true, mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictBudgetRefused {
		t.Fatalf("verdict = %s, want %s", result.Verdict, VerdictBudgetRefused)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("harness invoked %d times, want 0 — a refused budget must never reach the agent", len(mock.Calls))
	}
}

func TestOne_NoSeatIsSkipped(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{}
	step := workflow.Step{Department: "Marketing", Because: "campaign", Input: "artifact-abc123def456.json"}

	result, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", true, mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictNoSeat {
		t.Fatalf("verdict = %s, want %s", result.Verdict, VerdictNoSeat)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("harness invoked %d times, want 0", len(mock.Calls))
	}
}

// Acting is opt-in: without --execute, `hr run` reaches a decision and stops.
// A dispatcher that acts by default is one bad loop away from an invoice.
func TestOne_DryRunReachesADecisionAndInvokesNothing(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{Result: harness.Result{OK: true}}
	step := workflow.Step{Department: "QA", Because: "verify change", Input: "artifact-abc123def456.json"}

	result, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", false, mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictDryRun {
		t.Fatalf("verdict = %s, want %s", result.Verdict, VerdictDryRun)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("harness invoked %d times without --execute", len(mock.Calls))
	}
	// The decision still happened and was recorded: a dry run is a plan, not
	// a no-op, and the ledger is how you see what it would have done.
	entries, err := ledger.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	var sawDecision bool
	for _, e := range entries {
		if e.Kind == "decision" {
			sawDecision = true
		}
		if e.Kind == "action" {
			t.Errorf("a dry run recorded an action entry: %+v", e)
		}
	}
	if !sawDecision {
		t.Error("a dry run recorded no decision - it consulted policy but left no trace")
	}
}

// The cheap question first: an unaffordable step never reaches the policy
// engine or the model. Asking an expensive question after an unaffordable
// answer is waste.
func TestOne_BudgetRefusalHappensBeforePolicyAndTheModel(t *testing.T) {
	root := testRoot(t)
	writeGoalArtifact(t, root, "aaaaaaaaaaaa", "1000")
	mock := &harness.Mock{Result: harness.Result{OK: true}}
	step := workflow.Step{Department: "QA", Because: "verify change", Input: "goal aaaaaaaaaaaa — verify change"}

	result, err := One(context.Background(), persistence.File{Root: root}, root, step, "spiffe://hr.local/test", true, mock)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictBudgetRefused {
		t.Fatalf("verdict = %s, want %s", result.Verdict, VerdictBudgetRefused)
	}
	if result.Decision != nil {
		t.Error("policy was evaluated after the budget already refused - the cheap question must come first")
	}
	if len(mock.Calls) != 0 {
		t.Error("the model was invoked despite a budget refusal")
	}
}
