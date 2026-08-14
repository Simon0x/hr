package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/persistence"
)

func writeGoalArtifact(t *testing.T, root, outcome, budget string) {
	t.Helper()
	stmt := `{
		"_type": "https://in-toto.io/Statement/v1",
		"subject": [{"name": "goal", "digest": {"sha256": "abc123"}}],
		"predicateType": "https://hr.dev/goal/v1",
		"predicate": {"outcome": "` + outcome + `", "owner": "test", "measure": "test", "budget": "` + budget + `"}
	}`
	dir := filepath.Join(root, ".hr", "artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goal-test.json"), []byte(stmt), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFanOut_InvokesEveryLeadConcurrently(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{Result: harness.Result{OK: true, Output: "found nothing that breaks it"}}
	leads := []Lead{
		{Name: "a", Prompt: "prompt a", Grant: harness.Grant{Allow: []string{"Read"}}},
		{Name: "b", Prompt: "prompt b", Grant: harness.Grant{Allow: []string{"Read"}}},
		{Name: "c", Prompt: "prompt c", Grant: harness.Grant{Allow: []string{"Read"}}},
	}

	results, check, err := FanOut(context.Background(), persistence.File{Root: root}, root, leads, "", mock, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if check.Refused {
		t.Fatal("expected no budget check when goal is empty")
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("harness invoked %d times, want 3", len(mock.Calls))
	}
	for _, r := range results {
		if !r.Result.OK || r.Blocked {
			t.Errorf("lead %q: OK=%v Blocked=%v, want OK, not blocked", r.Lead.Name, r.Result.OK, r.Blocked)
		}
	}
}

func TestFanOut_BudgetRefusalSkipsInvocation(t *testing.T) {
	root := testRoot(t)
	writeGoalArtifact(t, root, "tiny-goal", "1000")
	mock := &harness.Mock{Result: harness.Result{OK: true}}
	leads := []Lead{
		{Name: "a", Prompt: "prompt a", Grant: harness.Grant{Allow: []string{"Read"}}},
		{Name: "b", Prompt: "prompt b", Grant: harness.Grant{Allow: []string{"Read"}}},
		{Name: "c", Prompt: "prompt c", Grant: harness.Grant{Allow: []string{"Read"}}},
	}

	results, check, err := FanOut(context.Background(), persistence.File{Root: root}, root, leads, "tiny-goal", mock, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !check.Refused {
		t.Fatalf("expected budget refusal, got %+v", check)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results, want 0 — nothing should have been spawned", len(results))
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("harness invoked %d times, want 0 — budget refusal must happen before any lead spawns", len(mock.Calls))
	}
}

func TestFanOut_MarksBlockedLeadsSeparatelyFromFailed(t *testing.T) {
	root := testRoot(t)
	mock := &harness.Mock{Result: harness.Result{OK: false, Output: "Error: permission denied for WebSearch"}}
	leads := []Lead{{Name: "a", Prompt: "prompt a", Grant: harness.Grant{Allow: []string{"WebSearch"}}}}

	results, _, err := FanOut(context.Background(), persistence.File{Root: root}, root, leads, "", mock, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Blocked {
		t.Fatalf("expected the lead to be marked Blocked, got %+v", results)
	}
}
