package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/workflow"
)

func cmdNext(root string, args []string) int {
	jsonOut := hasFlag(args, "json")

	storeDir := filepath.Join(root, ".hr", "artifacts")
	if _, err := os.Stat(storeDir); os.IsNotExist(err) {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Println("Nothing in the store. Start with a GOAL — see contracts/predicates/goal.schema.json.")
		}
		return 0
	}

	plan, err := workflow.Derive(context.Background(), persistence.File{Root: root}, root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if jsonOut {
		b, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	if len(plan.Ready) == 0 && len(plan.Blocked) == 0 {
		fmt.Printf("%d artifact(s), nothing outstanding.\n", plan.Artifacts)
		return 0
	}

	fmt.Printf("%d artifact(s) in the store\n\n", plan.Artifacts)

	if len(plan.Ready) > 0 {
		fmt.Println("ready")
		for _, s := range plan.Ready {
			fmt.Printf("  %-12s %s\n", s.Department, s.Because)
			fmt.Printf("  %s ← %s\n", pad12, s.Input)
		}
		fmt.Println()
	}

	if len(plan.Blocked) > 0 {
		fmt.Println("blocked  ← a gate is holding, not a queue")
		for _, s := range plan.Blocked {
			fmt.Printf("  %-12s %s\n", s.Department, s.Because)
			fmt.Printf("  %s %s\n", pad12, s.Blocked)
		}
		fmt.Println()
	}

	return 0
}

const pad12 = "            "
