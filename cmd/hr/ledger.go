package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
)

func cmdLedger(root string, args []string) int {
	if len(args) == 0 {
		return ledgerShow(root, nil)
	}
	switch args[0] {
	case "append":
		return ledgerAppend(root, args[1:])
	case "verify":
		return ledgerVerify(root)
	case "cost":
		return ledgerCost(root)
	default:
		return ledgerShow(root, args[1:])
	}
}

func flagList(args []string, name string) []string {
	v, ok := flagValue(args, name)
	if !ok || v == "" {
		return nil
	}
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func ledgerAppend(root string, args []string) int {
	kind, hasKind := flagValue(args, "kind")
	actor, hasActor := flagValue(args, "actor")
	if !hasKind || !hasActor {
		fmt.Fprintln(os.Stderr, "append needs --kind and --actor")
		return 2
	}

	e := ledger.Entry{Kind: kind, Actor: actor}
	if at, ok := flagValue(args, "at"); ok {
		e.At = at
	} else {
		e.At = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}

	in := flagList(args, "in")
	out := flagList(args, "out")
	if in != nil || out != nil {
		e.Artifacts = &ledger.Artifacts{In: emptyIfNil(in), Out: emptyIfNil(out)}
	}
	if v, ok := flagValue(args, "goal"); ok {
		e.Goal = v
	}
	if v, ok := flagValue(args, "policy"); ok {
		e.Policy = v
	}
	if v, ok := flagValue(args, "outcome"); ok {
		e.Outcome = v
	}
	if v, ok := flagValue(args, "detail"); ok {
		e.Detail = v
	}

	cost := ledger.Cost{}
	hasCost := false
	if v, ok := flagValue(args, "tokens"); ok {
		n, _ := strconv.ParseFloat(v, 64)
		cost.Tokens = &n
		hasCost = true
	}
	if v, ok := flagValue(args, "seconds"); ok {
		n, _ := strconv.ParseFloat(v, 64)
		cost.Seconds = &n
		hasCost = true
	}
	if v, ok := flagValue(args, "model"); ok {
		cost.Model = v
		hasCost = true
	}
	if hasCost {
		e.Cost = &cost
	}

	written, err := persistence.File{Root: root}.Append(context.Background(), e)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Printf("%d %s %s\n", written.Seq, written.Kind, written.Actor)
	return 0
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}

func ledgerVerify(root string) int {
	entries, err := persistence.File{Root: root}.Read(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(entries) == 0 {
		fmt.Println("ledger is empty")
		return 0
	}
	if msg, ok := ledger.VerifyChain(entries); !ok {
		fmt.Println(msg)
		return 1
	}
	fmt.Printf("chain intact — %d entr%s\n", len(entries), plural(len(entries), "y", "ies"))
	return 0
}

func ledgerCost(root string) int {
	entries, err := persistence.File{Root: root}.Read(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	type acc struct {
		tokens, seconds float64
		events          int
	}
	byGoal := map[string]*acc{}
	var order []string
	for _, e := range entries {
		goal := e.Goal
		if goal == "" {
			goal = "(no goal)"
		}
		a, ok := byGoal[goal]
		if !ok {
			a = &acc{}
			byGoal[goal] = a
			order = append(order, goal)
		}
		a.events++
		if e.Cost != nil {
			if e.Cost.Tokens != nil {
				a.tokens += *e.Cost.Tokens
			}
			if e.Cost.Seconds != nil {
				a.seconds += *e.Cost.Seconds
			}
		}
	}

	if len(byGoal) == 0 {
		fmt.Println("nothing recorded")
		return 0
	}

	fmt.Println("spend by goal")
	fmt.Println()
	for _, g := range order {
		a := byGoal[g]
		fmt.Printf("  %s\n", g)
		fmt.Printf("    %d event(s) · %g tokens · %gs\n", a.events, a.tokens, a.seconds)
	}
	fmt.Println("\nTokens are telemetry, not the unit of value. Divide by accepted outcomes.")
	return 0
}

func ledgerShow(root string, args []string) int {
	entries, err := persistence.File{Root: root}.Read(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}

	if jsonOut {
		b, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	if len(entries) == 0 {
		fmt.Println("ledger is empty")
		return 0
	}

	start := 0
	if len(entries) > 20 {
		start = len(entries) - 20
	}
	for _, e := range entries[start:] {
		arts := ""
		if e.Artifacts != nil && len(e.Artifacts.Out) > 0 {
			arts = " → " + strings.Join(e.Artifacts.Out, ",")
		}
		fmt.Printf("%4d  %s  %-12s %-9s %s%s\n", e.Seq, e.At, e.Kind, e.Outcome, e.Actor, arts)
		if e.Detail != "" {
			fmt.Printf("      %s\n", e.Detail)
		}
	}
	return 0
}
