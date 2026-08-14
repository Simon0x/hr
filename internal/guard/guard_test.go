package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Simon0x/hr/internal/policy"
)

func testPolicy(t *testing.T) (*policy.Policy, string) {
	t.Helper()
	root := repoRoot(t)
	p, _, err := policy.LoadPolicy(filepath.Join(root, "policies", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	return p, "testdigest"
}

func call(tool string, input map[string]any) Call {
	return Call{ToolName: tool, ToolInput: input, ToolUseID: "t1"}
}

func TestShapeOf_ScoresByWhatTheCallCanDo(t *testing.T) {
	cases := []struct {
		call    Call
		risk    string
		rev     string
		comment string
	}{
		{call("Read", map[string]any{"file_path": "x"}), "R0", "reversible", "reading changes nothing"},
		{call("Write", map[string]any{"file_path": "x"}), "R1", "revert", "a file write is revertable"},
		{call("Bash", map[string]any{"command": "go build ./..."}), "R2", "revert", "an ordinary shell command"},
		{call("Bash", map[string]any{"command": "rm -rf /important"}), "R3", "irreversible", "a recursive delete is not"},
		{call("Bash", map[string]any{"command": "git push --force origin main"}), "R3", "irreversible", "force push rewrites history"},
		{call("SomeNewTool", nil), "R1", "revert", "an unclassified tool is not assumed harmless"},
	}
	for _, c := range cases {
		got := ShapeOf(c.call)
		if got.Risk != c.risk || got.Reversibility != c.rev {
			t.Errorf("%s: got %s/%s, want %s/%s (%s)", c.call.ToolName, got.Risk, got.Reversibility, c.risk, c.rev, c.comment)
		}
	}
}

// The engine's own ceiling rule does the work: an irreversible call cannot
// be taken at the autonomy a step was granted.
func TestDecide_RefusesAnIrreversibleCallInsideAnAllowedStep(t *testing.T) {
	p, digest := testPolicy(t)
	ctx := Context{Actor: "spiffe://hr.local/test", Requested: "A2"}

	_, _, allowed := Decide(p, digest, ctx, call("Read", map[string]any{"file_path": "x"}))
	if !allowed {
		t.Error("a read must be allowed - the guard is not meant to stop ordinary work")
	}

	decision, shape, allowed := Decide(p, digest, ctx, call("Bash", map[string]any{"command": "rm -rf /srv/data"}))
	if allowed {
		t.Fatal("a recursive delete was allowed at A2")
	}
	if shape.Risk != "R3" {
		t.Errorf("risk = %s, want R3", shape.Risk)
	}
	if len(decision.Reasons) == 0 {
		t.Error("a refusal with no reason cannot be acted on")
	}
}

// Escalate has to deny: there is no human to escalate to inside a running
// invocation, and treating "someone should look at this" as permission is
// the failure the engine exists to prevent.
func TestDecide_EscalateDeniesRatherThanProceeds(t *testing.T) {
	p, digest := testPolicy(t)
	decision, _, allowed := Decide(p, digest, Context{Actor: "a", Requested: "A2"},
		call("Bash", map[string]any{"command": "rm -rf x"}))
	if decision.Verdict == "allow" {
		t.Skip("policy allowed it; nothing to assert about escalation")
	}
	if allowed {
		t.Errorf("verdict %q was treated as permission", decision.Verdict)
	}
}

func TestDescribe_SummarisesInputInsteadOfEchoingIt(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := Describe(call("Bash", map[string]any{"command": long}))
	if len(got) > 200 {
		t.Errorf("description is %d chars - tool input is untrusted text and must not be echoed whole", len(got))
	}
	if got := Describe(call("Read", map[string]any{"file_path": "/etc/passwd"})); !strings.Contains(got, "Read") {
		t.Errorf("description %q does not name the tool", got)
	}
}

func TestDenialLog_FoldsOnceAndIsRemoved(t *testing.T) {
	root := t.TempDir()
	token := NewToken()

	for i := 0; i < 2; i++ {
		if err := RecordDenial(root, token, Denial{Action: "Bash: rm -rf x", Verdict: "escalate"}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := TakeDenials(root, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d denials, want 2", len(got))
	}
	again, err := TakeDenials(root, token)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("denials survived the fold and would be replayed into the ledger: %v", again)
	}
}

func TestNewToken_IsPerInvocation(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		tok := NewToken()
		if seen[tok] {
			t.Fatal("token repeated - two concurrent leads would share a denial log")
		}
		seen[tok] = true
	}
}
