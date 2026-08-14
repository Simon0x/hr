package workflow

import (
	"strings"
	"testing"

	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func findStep(t *testing.T, steps []Step, department string) *Step {
	t.Helper()
	for i, s := range steps {
		if s.Department == department {
			return &steps[i]
		}
	}
	return nil
}

func containsStep(steps []Step, department, becauseSubstr string) bool {
	for _, s := range steps {
		if s.Department == department && strings.Contains(s.Because, becauseSubstr) {
			return true
		}
	}
	return false
}

// DUR-002 (docs/audit/REGISTER.md): a goal naming an open finding should
// route straight to Engineering - the "second queue" departments/engineering.md
// describes - not through Discovery. This is the confirming test that
// finding needed.
func TestDeriveFrom_GoalNamingOpenFinding_RoutesStraightToEngineering(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "goal1", Kind: "goal", Predicate: map[string]any{
			"outcome": "close SEC-001", "risk": "R1",
			"serves": map[string]any{"findingId": "SEC-001"},
		}},
	}
	plan, err := DeriveFrom(artifacts, map[string]bool{"SEC-001": true})
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Ready, "Engineering", "closes open finding SEC-001") {
		t.Fatalf("Ready = %+v, want an Engineering step closing SEC-001 directly (no Discovery hop)", plan.Ready)
	}
	if containsStep(plan.Ready, "Discovery", "") {
		t.Errorf("Ready = %+v, a goal naming an open finding should never route through Discovery", plan.Ready)
	}
}

func TestDeriveFrom_GoalNamingClosedFinding_IsBlockedNotRouted(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "goal1", Kind: "goal", Predicate: map[string]any{
			"outcome": "close it",
			"serves":  map[string]any{"findingId": "SEC-999"},
		}},
	}
	plan, err := DeriveFrom(artifacts, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Blocked, "Engineering", "not open in the register") {
		t.Fatalf("Blocked = %+v, want an Engineering entry naming SEC-999 as not open", plan.Blocked)
	}
}

func TestDeriveFrom_GoalNamingOpenFinding_AlreadyServedByChange_IsNotReAdded(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "goal1", Kind: "goal", Predicate: map[string]any{
			"serves": map[string]any{"findingId": "SEC-001"},
		}},
		{ID: "change1", Kind: "change", Predicate: map[string]any{
			"serves": map[string]any{"findingId": "SEC-001"},
		}},
	}
	plan, err := DeriveFrom(artifacts, map[string]bool{"SEC-001": true})
	if err != nil {
		t.Fatal(err)
	}
	if containsStep(plan.Ready, "Engineering", "closes open finding") {
		t.Errorf("Ready = %+v, a finding already served by a change should not be re-queued", plan.Ready)
	}
}

func TestDeriveFrom_ProblemWithNoHypothesis_RoutesToDiscovery(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "problem1", Kind: "problem", Predicate: map[string]any{"who": "ops team"}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Ready, "Discovery", "no hypothesis has been chained") {
		t.Fatalf("Ready = %+v, want a Discovery step for the unchained problem", plan.Ready)
	}
}

func TestDeriveFrom_HypothesisWithABreak_IsBlocked(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "hyp1", Kind: "hypothesis", Predicate: map[string]any{
			"claim": "x", "break": "mechanism",
			"chain": []any{
				map[string]any{"link": "mechanism", "status": "breaks", "rung": "desk"},
			},
		}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Blocked, "Discovery", `"mechanism" breaks`) {
		t.Fatalf("Blocked = %+v, want a Discovery entry naming the broken link", plan.Blocked)
	}
}

func TestDeriveFrom_HypothesisBelowGateRung_IsBlockedToProduct(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "hyp1", Kind: "hypothesis", Predicate: map[string]any{
			"claim": "x",
			"chain": []any{
				map[string]any{"link": "a", "status": "holds", "rung": "desk"},
			},
		}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Blocked, "Product", "rests on `desk`") {
		t.Fatalf("Blocked = %+v, want a Product entry naming the desk rung as below the behaviour gate", plan.Blocked)
	}
}

func TestDeriveFrom_HypothesisAtGateRung_IsReadyWithMappedConfidence(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "hyp1", Kind: "hypothesis", Predicate: map[string]any{
			"claim": "legacy operators are losing share",
			"chain": []any{
				map[string]any{"link": "a", "status": "holds", "rung": "behaviour"},
			},
		}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	step := findStep(t, plan.Ready, "Product")
	if step == nil {
		t.Fatalf("Ready = %+v, want a Product step for the hypothesis at gate rung", plan.Ready)
	}
	if step.Confidence != "likely" {
		t.Errorf("confidence = %q, want %q (confidenceFor(\"behaviour\"))", step.Confidence, "likely")
	}
}

func TestDeriveFrom_SpecWithNoChange_IsReadyWithConfidenceFromItsChainLink(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "hyp1", Kind: "hypothesis", Predicate: map[string]any{
			"chain": []any{map[string]any{"link": "a", "status": "holds", "rung": "revenue"}},
		}},
		{ID: "spec1", Kind: "spec", Predicate: map[string]any{
			"id": "SPEC-1", "intent": "do the thing",
			"serves": map[string]any{"hypothesisId": "hyp1", "link": "a"},
		}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	step := findStep(t, plan.Ready, "Engineering")
	if step == nil || !strings.Contains(step.Input, "SPEC-1") {
		t.Fatalf("Ready = %+v, want an Engineering step for SPEC-1", plan.Ready)
	}
	if step.Confidence != "certain" {
		t.Errorf("confidence = %q, want %q (confidenceFor(\"revenue\"), read off the serving hypothesis's chain link)", step.Confidence, "certain")
	}
}

func TestDeriveFrom_ChangeWithNoVerdict_IsReadyForQA(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "change1", Kind: "change",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "changedigest"}}},
			Predicate: map[string]any{"rung": "revert", "serves": map[string]any{"specId": "SPEC-1"}},
		},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	step := findStep(t, plan.Ready, "QA")
	if step == nil {
		t.Fatalf("Ready = %+v, want a QA step for the ungraded change", plan.Ready)
	}
	if step.Reversibility != "revert" {
		t.Errorf("reversibility = %q, want %q", step.Reversibility, "revert")
	}
}

func TestDeriveFrom_VerdictAllCriteriaMet_NoRelease_IsReadyForRelease(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "verdict1", Kind: "verdict",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "verdictsubj"}}},
			Predicate: map[string]any{"acceptance": []any{map[string]any{"state": "met"}}},
		},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Ready, "Release", "every criterion met") {
		t.Fatalf("Ready = %+v, want a Release step for the fully-met verdict", plan.Ready)
	}
}

func TestDeriveFrom_VerdictWithUnmetCriteria_NoFixInFlight_IsReadyForEngineering(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "change1", Kind: "change",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "changedigest"}}},
			Predicate: map[string]any{"rung": "revert", "serves": map[string]any{"specId": "SPEC-1"}},
		},
		{ID: "verdict1", Kind: "verdict",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "changedigest"}}},
			Predicate: map[string]any{"acceptance": []any{map[string]any{"state": "unmet"}}},
		},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Ready, "Engineering", "1 unmet criterion") {
		t.Fatalf("Ready = %+v, want an Engineering step for the unmet criterion", plan.Ready)
	}
}

func TestDeriveFrom_VerdictWithUnmetCriteria_FixAlreadyInFlight_IsSuppressed(t *testing.T) {
	artifacts := []store.Artifact{
		// change1 is what verdict1 graded - unmet.
		{ID: "change1", Kind: "change",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "change1digest"}}},
			Predicate: map[string]any{"rung": "revert", "serves": map[string]any{"specId": "SPEC-1"}},
		},
		{ID: "verdict1", Kind: "verdict",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "change1digest"}}},
			Predicate: map[string]any{"acceptance": []any{map[string]any{"state": "unmet"}}},
		},
		// change2 already serves the same spec and has no verdict yet - the fix is in flight.
		{ID: "change2", Kind: "change",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "change2digest"}}},
			Predicate: map[string]any{"rung": "revert", "serves": map[string]any{"specId": "SPEC-1"}},
		},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsStep(plan.Ready, "Engineering", "unmet") {
		t.Errorf("Ready = %+v, a verdict whose fix is already in flight (change2) should not re-queue Engineering", plan.Ready)
	}
}

func TestDeriveFrom_VerdictWithUnverifiableCriteria_IsReadyForProduct(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "verdict1", Kind: "verdict",
			Subject:   []statement.Subject{{Digest: map[string]string{"sha256": "verdictsubj"}}},
			Predicate: map[string]any{"acceptance": []any{map[string]any{"state": "unverifiable"}}},
		},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Ready, "Product", "unverifiable criterion") {
		t.Fatalf("Ready = %+v, want a Product step naming the unverifiable criterion", plan.Ready)
	}
}

func TestDeriveFrom_ReleaseRollbackNotRehearsed_IsBlocked(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "release1", Kind: "release", Predicate: map[string]any{"rollbackRehearsed": false, "artifactDigest": "x"}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Blocked, "Release", "rollback path has not been rehearsed") {
		t.Fatalf("Blocked = %+v, want a Release entry for the unrehearsed rollback", plan.Blocked)
	}
}

func TestDeriveFrom_ReleaseRollbackRehearsed_IsReadyForOps(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "release1", Kind: "release", Predicate: map[string]any{"rollbackRehearsed": true, "artifactDigest": "x"}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStep(plan.Ready, "Ops", "nothing is watching it yet") {
		t.Fatalf("Ready = %+v, want an Ops step for the live, unwatched release", plan.Ready)
	}
}

func TestDeriveFrom_ArtifactCountIsReported(t *testing.T) {
	artifacts := []store.Artifact{
		{ID: "a1", Kind: "problem", Predicate: map[string]any{}},
		{ID: "a2", Kind: "hypothesis", Predicate: map[string]any{}},
	}
	plan, err := DeriveFrom(artifacts, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Artifacts != 2 {
		t.Errorf("Artifacts = %d, want 2", plan.Artifacts)
	}
}
