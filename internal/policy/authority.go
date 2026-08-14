package policy

import (
	"encoding/json"
	"fmt"
	"os"
)

var autonomyOrder = []string{"A0", "A1", "A2", "A3", "A4"}

type Dimensions struct {
	Risk          string `json:"risk"`
	Reversibility string `json:"reversibility"`
	Autonomy      string `json:"autonomy"`
	Confidence    string `json:"confidence"`
	Observability string `json:"observability"`
}

type Decision struct {
	Action     string     `json:"action"`
	Verdict    string     `json:"verdict"`
	Policy     string     `json:"policy"`
	Dimensions Dimensions `json:"dimensions"`
	Reasons    []string   `json:"reasons"`
}

type Rule struct {
	ID      string         `json:"id"`
	When    map[string]any `json:"when"`
	Ceiling string         `json:"ceiling,omitempty"`
	Verdict string         `json:"verdict,omitempty"`
	Because string         `json:"because"`
}

type Policy struct {
	Version  string            `json:"version"`
	Ceilings map[string]string `json:"ceilings"`
	Rules    []Rule            `json:"rules"`
	Autonomy map[string]string `json:"autonomy"`
}

func LoadPolicy(path string) (*Policy, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var p Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, nil, err
	}
	return &p, raw, nil
}

type Facts struct {
	Action            string
	Actor             string
	Risk              string
	Reversibility     string
	Confidence        string
	Observability     string
	RollbackRehearsed bool
}

func indexOf(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}

func applies(when map[string]any, facts map[string]any) bool {
	for k, v := range when {
		fv := facts[k]
		if list, ok := v.([]any); ok {
			found := false
			for _, item := range list {
				if fmt.Sprint(item) == fmt.Sprint(fv) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
			continue
		}
		if fmt.Sprint(v) != fmt.Sprint(fv) {
			return false
		}
	}
	return true
}

func Evaluate(p *Policy, policyDigest string, facts Facts, requested string) Decision {
	factsMap := map[string]any{
		"action": facts.Action, "actor": facts.Actor, "risk": facts.Risk,
		"reversibility": facts.Reversibility, "confidence": facts.Confidence,
		"observability": facts.Observability, "rollbackRehearsed": facts.RollbackRehearsed,
	}

	ceiling := p.Ceilings[facts.Risk]
	if ceiling == "" {
		ceiling = "A0"
	}
	verdict := "allow"
	var reasons []string
	reasons = append(reasons, fmt.Sprintf("%s caps autonomy at %s — %s", facts.Risk, ceiling, p.Autonomy[ceiling]))

	for _, rule := range p.Rules {
		if !applies(rule.When, factsMap) {
			continue
		}
		if rule.Verdict == "refuse" {
			verdict = "refuse"
			reasons = append(reasons, fmt.Sprintf("%s: %s", rule.ID, rule.Because))
			continue
		}
		if rule.Ceiling != "" && indexOf(autonomyOrder, rule.Ceiling) < indexOf(autonomyOrder, ceiling) {
			ceiling = rule.Ceiling
			reasons = append(reasons, fmt.Sprintf("%s lowers the ceiling to %s: %s", rule.ID, ceiling, rule.Because))
		}
	}

	if verdict != "refuse" {
		if indexOf(autonomyOrder, requested) > indexOf(autonomyOrder, ceiling) {
			verdict = "escalate"
			reasons = append(reasons, fmt.Sprintf(
				"requested %s exceeds the ceiling %s — a human decides, and the work resumes once they have",
				requested, ceiling))
		} else {
			reasons = append(reasons, fmt.Sprintf("requested %s is within the ceiling %s", requested, ceiling))
		}
	}

	return Decision{
		Action:  facts.Action,
		Verdict: verdict,
		Policy:  policyDigest,
		Dimensions: Dimensions{
			Risk: facts.Risk, Reversibility: facts.Reversibility, Autonomy: ceiling,
			Confidence: facts.Confidence, Observability: facts.Observability,
		},
		Reasons: reasons,
	}
}

func ExitCode(verdict string) int {
	switch verdict {
	case "allow":
		return 0
	case "escalate":
		return 3
	default:
		return 1
	}
}
