package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/Simon0x/hr/internal/budget"
	"github.com/Simon0x/hr/internal/persistence"
)

func cmdBudget(root string, args []string) int {
	if len(args) == 0 {
		return budgetReport(root)
	}
	switch args[0] {
	case "check":
		return budgetCheck(root, args[1:])
	default:
		return budgetReport(root)
	}
}

func commaFormat(n float64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	whole := int64(n)
	s := strconv.FormatInt(whole, 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	result := string(out)
	if frac := n - float64(whole); frac > 0 {
		result += strings.TrimPrefix(fmt.Sprintf("%.3f", frac), "0")
	}
	if neg {
		result = "-" + result
	}
	return result
}

func budgetCheck(root string, args []string) int {
	goalID, ok := flagValue(args, "goal")
	if !ok {
		fmt.Fprintln(os.Stderr, "check needs --goal")
		return 2
	}

	estimate := 0.0
	hasEstimate := false
	if v, ok := flagValue(args, "estimate"); ok {
		estimate, _ = strconv.ParseFloat(v, 64)
		hasEstimate = true
	}

	result, err := budget.Check(context.Background(), persistence.File{Root: root}, root, goalID, estimate)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if !result.HasBudget {
		fmt.Printf("no budget on %s\n", goalID)
		fmt.Println("an unbounded goal cannot be traded off against another")
		return 0
	}

	fmt.Println(goalID)
	fmt.Printf("  %-10s%s tokens\n", "budget", commaFormat(result.Limit))
	fmt.Printf("  %-10s%s\n", "spent", commaFormat(result.Spent))
	if hasEstimate {
		fmt.Printf("  %-10s%s\n", "estimate", commaFormat(result.Estimate))
	}
	fmt.Printf("  %-10s%s (%d%%)\n", "after", commaFormat(result.After), result.Percent)

	if result.Refused {
		fmt.Printf("\nREFUSED — this would exceed the budget by %s tokens.\n\n", commaFormat(result.After-result.Limit))
		fmt.Println("  Raise the budget deliberately or cut the scope. Silently continuing is\n" +
			"  how a 2.4x overshoot happens: every individual step looked affordable.")
		return 1
	}
	if result.Warn {
		fmt.Printf("\nWARN — %d%% of budget committed. Decide now, not at 100%%.\n", result.Percent)
		return 0
	}
	fmt.Println("\nOK")
	return 0
}

func budgetReport(root string) int {
	artifacts, err := persistence.File{Root: root}.Load(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	entries, err := persistence.File{Root: root}.Read(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	costs := budget.Report(artifacts, entries)
	if len(costs) == 0 {
		fmt.Println("nothing to report — no goal has been touched yet")
		return 0
	}

	fmt.Println("cost per outcome")
	for _, c := range costs {
		perAccepted := "— no outcome recorded yet, so this is spend with no denominator"
		if c.Outcomes > 0 {
			perAccepted = "— nothing accepted, so all of it was overhead"
		}
		if c.Denominated {
			perAccepted = commaFormat(math.Round(c.PerAccepted)) + " tokens"
		}

		fmt.Printf("\n  %s\n", c.Goal)
		fmt.Printf("    %-17s%s tokens\n", "spent", commaFormat(c.Spent))
		fmt.Printf("    %-17s%d (%d accepted)\n", "outcomes", c.Outcomes, c.Accepted)
		fmt.Printf("    %-17s%s\n", "per accepted", perAccepted)
		fmt.Printf("    %-17s%s\n", "human touches", commaFormat(c.HumanTouches))
	}

	fmt.Println("\nTokens are telemetry. Cost per accepted outcome is the number, and human\n" +
		"touches is the one that decides whether spend falling is actually progress -\n" +
		"spend can drop while people do more of the work, which is not an improvement.")
	return 0
}
