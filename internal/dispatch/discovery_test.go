package dispatch

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/workflow"
)

const testHypothesisFixture = `{
	"_type": "https://in-toto.io/Statement/v1",
	"subject": [{"name": "hypothesis", "digest": {"sha256": "abc123"}}],
	"predicateType": "https://hr.dev/hypothesis/v1",
	"predicate": {
		"claim": "test claim",
		"chain": [
			{"link": "structural", "status": "holds", "rung": "desk", "source": "test lead"}
		],
		"break": ""
	}
}`

func TestRunDiscovery_AllLeadsBlocked_FilesOneExceptionAndSkipsSynthesis(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{Result: harness.Result{OK: false, Output: "Error: permission denied for WebSearch"}}
	step := workflow.Step{Department: "Discovery", Because: "test the claim", Input: "claim: legacy operators are losing share to feature gaps"}
	result := StepResult{Step: step}

	got, err := runDiscovery(context.Background(), persistence.File{Root: root}, root, step, StepKey(step), "spiffe://hr.local/test", result, mock, Seat{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != VerdictBlocked {
		t.Fatalf("verdict = %s, want %s", got.Verdict, VerdictBlocked)
	}
	if got.ExceptionPath == "" {
		t.Fatalf("expected an exception artifact path, got exception err: %v", got.ExceptionErr)
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("harness invoked %d times, want exactly 3 (one per lead, no synthesis or verify)", len(mock.Calls))
	}
	for _, c := range mock.Calls {
		if c.Schema != "" || strings.Contains(c.Prompt, "Test this claim:") {
			t.Errorf("call %+v looks like synthesis or verify — should never run when every lead is blocked", c)
		}
	}
}

func TestRunDiscovery_SettledLeadsSynthesizeAndVerify(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{
		Result:           harness.Result{OK: true, Output: "this angle holds, evidence at desk rung"},
		StructuredOutput: json.RawMessage(testHypothesisFixture),
	}
	step := workflow.Step{Department: "Discovery", Because: "test the claim", Input: "claim: legacy operators are losing share to feature gaps"}
	result := StepResult{Step: step}

	got, err := runDiscovery(context.Background(), persistence.File{Root: root}, root, step, StepKey(step), "spiffe://hr.local/test", result, mock, Seat{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Verdict != VerdictInvoked {
		t.Fatalf("verdict = %s, want %s (agent output: %s)", got.Verdict, VerdictInvoked, got.AgentOutput)
	}

	var sawStructured, sawVerify int
	for _, c := range mock.Calls {
		if c.Schema != "" {
			sawStructured++
		}
		if strings.Contains(c.Prompt, "Test this claim:") {
			sawVerify++
			if !strings.Contains(c.Prompt, "You test claims.") {
				t.Errorf("verify call prompt missing agents/validate.md's instructions: %q", c.Prompt)
			}
		}
	}
	if sawStructured != 1 {
		t.Errorf("saw %d structured (synthesis) calls, want 1", sawStructured)
	}
	if sawVerify != 1 {
		t.Errorf("saw %d verify calls (agents/validate.md invoked as a plain Invoke), want 1", sawVerify)
	}
	if len(mock.Calls) != 5 {
		t.Errorf("harness invoked %d times, want 5 (3 leads + synthesis + verify)", len(mock.Calls))
	}

	matches, _ := filepath.Glob(filepath.Join(root, ".hr", "artifacts", "hypothesis-*.json"))
	if len(matches) != 1 {
		t.Errorf("found %d hypothesis artifacts on disk, want 1", len(matches))
	}
}
