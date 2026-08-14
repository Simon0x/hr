package policy

import (
	"path/filepath"
	"testing"
)

func testPolicy() *Policy {
	return &Policy{
		Version:  "1",
		Ceilings: map[string]string{"R0": "A4", "R1": "A3", "R2": "A2", "R3": "A1", "R4": "A0"},
		Rules: []Rule{
			{ID: "silent-caps", When: map[string]any{"observability": []any{"silent"}}, Ceiling: "A1", Because: "test"},
			{ID: "unknown-confidence-caps", When: map[string]any{"confidence": []any{"unknown"}}, Ceiling: "A1", Because: "test"},
			{ID: "unattributed-refused", When: map[string]any{"actor": []any{"", "unattributed"}}, Verdict: "refuse", Because: "test"},
			{ID: "would-raise-ceiling", When: map[string]any{"risk": []any{"R4"}}, Ceiling: "A4", Because: "must never actually raise the ceiling"},
		},
		Autonomy: map[string]string{"A0": "recommend", "A1": "prepare", "A2": "execute internal", "A3": "execute external", "A4": "unattended"},
	}
}

func TestEvaluate_WithinCeiling_Allows(t *testing.T) {
	p := testPolicy()
	facts := Facts{Action: "test", Actor: "spiffe://hr.local/test", Risk: "R0", Reversibility: "revert", Confidence: "certain", Observability: "refuses"}
	d := Evaluate(p, "digest", facts, "A2")
	if d.Verdict != "allow" {
		t.Fatalf("verdict = %s, want allow (requested A2 within R0's A4 ceiling): %v", d.Verdict, d.Reasons)
	}
	if d.Dimensions.Autonomy != "A4" {
		t.Errorf("ceiling = %s, want A4 (R0's ceiling, no rule applies)", d.Dimensions.Autonomy)
	}
}

func TestEvaluate_RequestedAboveCeiling_Escalates(t *testing.T) {
	p := testPolicy()
	facts := Facts{Action: "test", Actor: "spiffe://hr.local/test", Risk: "R4", Reversibility: "revert", Confidence: "certain", Observability: "refuses"}
	d := Evaluate(p, "digest", facts, "A2")
	if d.Verdict != "escalate" {
		t.Fatalf("verdict = %s, want escalate (requested A2 exceeds R4's A0 ceiling): %v", d.Verdict, d.Reasons)
	}
}

func TestEvaluate_RefuseRuleShortCircuits(t *testing.T) {
	p := testPolicy()
	facts := Facts{Action: "test", Actor: "", Risk: "R0", Reversibility: "revert", Confidence: "certain", Observability: "refuses"}
	d := Evaluate(p, "digest", facts, "A0")
	if d.Verdict != "refuse" {
		t.Fatalf("verdict = %s, want refuse (unattributed actor), reasons: %v", d.Verdict, d.Reasons)
	}
}

func TestEvaluate_MultipleLoweringRulesComposeToTheStrictest(t *testing.T) {
	p := testPolicy()
	// R0's baseline ceiling is A4, but silent observability and unknown
	// confidence each independently lower it to A1 - the strictest applicable
	// rule governs, per policies/README.md.
	facts := Facts{Action: "test", Actor: "spiffe://hr.local/test", Risk: "R0", Reversibility: "revert", Confidence: "unknown", Observability: "silent"}
	d := Evaluate(p, "digest", facts, "A2")
	if d.Dimensions.Autonomy != "A1" {
		t.Fatalf("ceiling = %s, want A1 (two rules each lower R0's A4 ceiling to A1): %v", d.Dimensions.Autonomy, d.Reasons)
	}
	if d.Verdict != "escalate" {
		t.Fatalf("verdict = %s, want escalate (requested A2 exceeds the lowered A1 ceiling)", d.Verdict)
	}
}

func TestEvaluate_ARuleNeverRaisesTheCeiling(t *testing.T) {
	p := testPolicy()
	// would-raise-ceiling names A4 for R4, but R4's own baseline ceiling is
	// already A0 - a rule may only lower the ceiling, never raise it, so A0
	// must survive untouched even though a rule matched.
	facts := Facts{Action: "test", Actor: "spiffe://hr.local/test", Risk: "R4", Reversibility: "revert", Confidence: "certain", Observability: "refuses"}
	d := Evaluate(p, "digest", facts, "A0")
	if d.Dimensions.Autonomy != "A0" {
		t.Fatalf("ceiling = %s, want A0 (a rule naming a higher ceiling must never raise it): %v", d.Dimensions.Autonomy, d.Reasons)
	}
}

func TestEvaluate_PolicyDigestPassesThroughUnchanged(t *testing.T) {
	p := testPolicy()
	d := Evaluate(p, "abc123digest", Facts{Action: "test", Risk: "R0"}, "A0")
	if d.Policy != "abc123digest" {
		t.Errorf("policy digest = %s, want abc123digest", d.Policy)
	}
}

func TestExitCode(t *testing.T) {
	cases := map[string]int{"allow": 0, "escalate": 3, "refuse": 1, "anything-else": 1}
	for verdict, want := range cases {
		if got := ExitCode(verdict); got != want {
			t.Errorf("ExitCode(%q) = %d, want %d", verdict, got, want)
		}
	}
}

// Regression coverage against the policy actually shipped in this repo, not
// just a synthetic fixture - catches a rule edit in policies/default.json
// breaking the property the README documents.
func TestEvaluate_AgainstShippedDefaultPolicy_UnattributedIsRefused(t *testing.T) {
	p, _, err := LoadPolicy(filepath.Join("..", "..", "policies", "default.json"))
	if err != nil {
		t.Fatal(err)
	}
	facts := Facts{Action: "test", Actor: "", Risk: "R1", Reversibility: "revert", Confidence: "certain", Observability: "refuses"}
	d := Evaluate(p, "digest", facts, "A2")
	if d.Verdict != "refuse" {
		t.Fatalf("verdict = %s, want refuse (default.json's unattributed-is-refused rule), reasons: %v", d.Verdict, d.Reasons)
	}
}
