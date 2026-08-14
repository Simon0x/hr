package budget

import (
	"testing"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/store"
)

func outcome(goal string, accepted bool, touches float64) store.Artifact {
	return store.Artifact{Kind: "outcome", Predicate: map[string]any{
		"goal": goal, "accepted": accepted, "humanTouches": touches,
	}}
}

func spend(goal string, tokens float64) ledger.Entry {
	return ledger.Entry{Kind: "action", Actor: "a", Goal: goal, Outcome: "ok",
		Cost: &ledger.Cost{Tokens: &tokens}}
}

// The constraint: cost is reported per accepted outcome, never as raw tokens.
func TestReport_DividesSpendByAcceptedOutcomes(t *testing.T) {
	got := Report(
		[]store.Artifact{outcome("g1", true, 2), outcome("g1", true, 1), outcome("g1", false, 4)},
		[]ledger.Entry{spend("g1", 900)},
	)
	if len(got) != 1 {
		t.Fatalf("got %d goals, want 1", len(got))
	}
	c := got[0]
	if c.Spent != 900 || c.Outcomes != 3 || c.Accepted != 2 {
		t.Fatalf("spent=%v outcomes=%d accepted=%d", c.Spent, c.Outcomes, c.Accepted)
	}
	if !c.Denominated || c.PerAccepted != 450 {
		t.Errorf("per accepted = %v (denominated %v), want 450 — spend over *accepted*, not over all outcomes",
			c.PerAccepted, c.Denominated)
	}
	// Human touches sits beside it: spend can fall while people do more of
	// the work, which is not an improvement.
	if c.HumanTouches != 7 {
		t.Errorf("human touches = %v, want 7 (counted across every outcome, accepted or not)", c.HumanTouches)
	}
}

// Spend with nothing accepted must not be reported as a cost figure: a
// denominator of zero is absent, not one.
func TestReport_RefusesToDivideByNothingAccepted(t *testing.T) {
	got := Report(
		[]store.Artifact{outcome("g1", false, 0)},
		[]ledger.Entry{spend("g1", 5000)},
	)
	if got[0].Denominated {
		t.Error("reported a per-accepted figure with nothing accepted — that is raw spend wearing a rate's clothes")
	}
	if got[0].PerAccepted != 0 {
		t.Errorf("per accepted = %v, want 0 when undenominated", got[0].PerAccepted)
	}

	none := Report(nil, []ledger.Entry{spend("g2", 100)})
	if len(none) != 1 || none[0].Denominated {
		t.Errorf("a goal with no outcomes at all must report spend with no denominator: %+v", none)
	}
}

func TestReport_CoversEveryGoalMentionedByEitherSide(t *testing.T) {
	got := Report(
		[]store.Artifact{outcome("only-an-outcome", true, 0)},
		[]ledger.Entry{spend("only-spend", 10)},
	)
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.Goal] = true
	}
	if !seen["only-spend"] || !seen["only-an-outcome"] {
		t.Errorf("goals = %+v, want both — a goal with spend and no outcome is the case worth seeing", got)
	}
}
