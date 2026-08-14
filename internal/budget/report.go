package budget

import (
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/store"
)

// GoalCost is what a goal spent and what it has to show for it.
//
// Spent alone is telemetry. PerAccepted is the number, and it only exists
// when something was accepted: a denominator of zero is reported as absent
// rather than as a spend figure, because "12,000 tokens" with nothing
// accepted reads like a cost when it is entirely overhead. HumanTouches sits
// beside it because spend can fall while people do more of the work, which is
// not an improvement.
type GoalCost struct {
	Goal         string
	Spent        float64
	Outcomes     int
	Accepted     int
	HumanTouches float64
	// PerAccepted is spend divided by accepted outcomes, and Denominated
	// reports whether that division was possible at all.
	PerAccepted float64
	Denominated bool
}

// Report computes cost per accepted outcome for every goal any entry or
// outcome mentions, in first-seen order.
func Report(artifacts []store.Artifact, entries []ledger.Entry) []GoalCost {
	var outcomes []store.Artifact
	for _, a := range artifacts {
		if a.Kind == "outcome" {
			outcomes = append(outcomes, a)
		}
	}

	seen := map[string]bool{}
	var goals []string
	add := func(g string) {
		if g == "" || seen[g] {
			return
		}
		seen[g] = true
		goals = append(goals, g)
	}
	for _, e := range entries {
		add(e.Goal)
	}
	for _, o := range outcomes {
		g, _ := o.Predicate["goal"].(string)
		add(g)
	}

	out := make([]GoalCost, 0, len(goals))
	for _, g := range goals {
		c := GoalCost{Goal: g, Spent: SpentOn(entries, g)}
		for _, o := range outcomes {
			if og, _ := o.Predicate["goal"].(string); og != g {
				continue
			}
			c.Outcomes++
			if a, _ := o.Predicate["accepted"].(bool); a {
				c.Accepted++
			}
			if t, ok := o.Predicate["humanTouches"].(float64); ok {
				c.HumanTouches += t
			}
		}
		if c.Accepted > 0 {
			c.PerAccepted = c.Spent / float64(c.Accepted)
			c.Denominated = true
		}
		out = append(out, c)
	}
	return out
}
