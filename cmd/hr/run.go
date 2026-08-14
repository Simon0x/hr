package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Simon0x/hr/internal/dispatch"
	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/workflow"
)

func hrActor() string {
	if a := os.Getenv("HR_ACTOR"); a != "" {
		return a
	}
	return "spiffe://hr.local/dispatcher"
}

func cmdRun(root string, args []string) int {
	execute := hasFlag(args, "execute")
	max := 1
	if v, ok := flagValue(args, "max"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			max = n
		}
	}

	h, err := harness.Select(os.Getenv("HR_HARNESS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	plan, err := workflow.Derive(context.Background(), persistence.File{Root: root}, root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if len(plan.Ready) == 0 {
		fmt.Println("nothing ready.")
		for _, b := range plan.Blocked {
			fmt.Printf("  blocked  %s — %s\n           %s\n", b.Department, b.Because, b.Blocked)
		}
		return 0
	}

	if !execute {
		fmt.Println("DRY RUN — nothing will be invoked. Pass --execute to act.")
		fmt.Println()
	}

	steps := plan.Ready
	if len(steps) > max {
		steps = steps[:max]
	}

	done := 0
	ctx := context.Background()
	actor := hrActor()

	for _, step := range steps {
		fmt.Printf("%s — %s\n", step.Department, step.Because)
		fmt.Printf("  input   %s\n", step.Input)

		result, err := dispatch.One(ctx, persistence.File{Root: root}, root, step, actor, execute, h)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR   %v\n\n", err)
			continue
		}

		if !result.HasSeat {
			fmt.Printf("  SKIP    no procedure for %s\n\n", step.Department)
			continue
		}

		fmt.Printf("  facts   risk %s · reversibility %s · confidence %s\n", result.Risk, result.Reversibility, result.Confidence)

		if result.Verdict == dispatch.VerdictBudgetRefused {
			fmt.Println("  REFUSED budget exceeded")
			fmt.Println()
			continue
		}

		d := result.Decision
		fmt.Printf("  policy  %s (%s)\n", d.Verdict, d.Policy)
		for _, r := range d.Reasons {
			fmt.Printf("          · %s\n", r)
		}

		switch result.Verdict {
		case dispatch.VerdictEscalated:
			if result.ExceptionPath != "" {
				fmt.Printf("  STOP    needs a human — filed %s\n\n", result.ExceptionPath)
			} else {
				fmt.Printf("  STOP    needs a human. Resolve, then run again. (failed to file exception: %v)\n\n", result.ExceptionErr)
			}
			continue
		case dispatch.VerdictRefused:
			fmt.Println("  STOP    refused.")
			fmt.Println()
			continue
		case dispatch.VerdictBlocked:
			if result.ExceptionPath != "" {
				fmt.Printf("  STOP    blocked on tool permission, not evidence — filed %s\n\n", result.ExceptionPath)
			} else {
				fmt.Printf("  STOP    blocked on tool permission. (failed to file exception: %v)\n\n", result.ExceptionErr)
			}
			continue
		}

		if !execute {
			fmt.Printf("  would   invoke %s\n\n", result.Seat.Procedure)
			continue
		}

		fmt.Printf("  invoke  %s\n", result.Seat.Procedure)
		status := "done"
		if !result.AgentOK {
			status = "FAILED"
		}
		fmt.Printf("  %s   exit %d\n", status, result.AgentExit)
		if !result.AgentOK {
			lines := strings.Split(result.AgentOutput, "\n")
			if len(lines) > 5 {
				lines = lines[:5]
			}
			for _, l := range lines {
				fmt.Printf("          %s\n", l)
			}
		}
		done++
		fmt.Println()
	}

	if execute {
		fmt.Printf("%d step(s) invoked. Run again for the next.\n", done)
	}
	return 0
}
