package budget

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/store"
)

var budgetPattern = regexp.MustCompile(`(?i)([\d.]+)\s*([km])?`)

func ParseBudget(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	m := budgetPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	mult := 1.0
	switch strings.ToLower(m[2]) {
	case "m":
		mult = 1e6
	case "k":
		mult = 1e3
	}
	return n * mult, true
}

func SpentOn(entries []ledger.Entry, goal string) float64 {
	var sum float64
	for _, e := range entries {
		if e.Goal == goal && e.Cost != nil && e.Cost.Tokens != nil {
			sum += *e.Cost.Tokens
		}
	}
	return sum
}

func FindGoal(artifacts []store.Artifact, goalID string) *store.Artifact {
	for i := range artifacts {
		a := artifacts[i]
		if a.Kind != "goal" {
			continue
		}
		if a.ID == goalID {
			return &artifacts[i]
		}
		if s, _ := a.Predicate["outcome"].(string); s == goalID {
			return &artifacts[i]
		}
		if s, _ := a.Predicate["owner"].(string); s == goalID {
			return &artifacts[i]
		}
	}
	return nil
}

type CheckResult struct {
	HasBudget bool
	Refused   bool
	Warn      bool
	Limit     float64
	Spent     float64
	Estimate  float64
	After     float64
	Percent   int
}

func Check(ctx context.Context, st persistence.Store, root, goalID string, estimate float64) (CheckResult, error) {
	artifacts, err := st.Load(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	entries, err := st.Read(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	return CheckFrom(artifacts, entries, goalID, estimate), nil
}

func CheckFrom(artifacts []store.Artifact, entries []ledger.Entry, goalID string, estimate float64) CheckResult {
	goal := FindGoal(artifacts, goalID)
	var budgetStr string
	if goal != nil {
		budgetStr, _ = goal.Predicate["budget"].(string)
	}
	limit, hasBudget := ParseBudget(budgetStr)
	spent := SpentOn(entries, goalID)

	if !hasBudget {
		return CheckResult{HasBudget: false, Spent: spent, Estimate: estimate}
	}

	after := spent + estimate
	pct := int(math.Round(after / limit * 100))
	return CheckResult{
		HasBudget: true, Refused: after > limit, Warn: pct >= 80,
		Limit: limit, Spent: spent, Estimate: estimate, After: after, Percent: pct,
	}
}
