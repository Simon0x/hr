package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/policy"
)

func cmdAuthority(root string, args []string) int {
	policyPath, ok := flagValue(args, "policy")
	if !ok {
		policyPath = filepath.Join(root, "policies", "default.json")
	}

	p, raw, err := policy.LoadPolicy(policyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	sum := sha256.Sum256(raw)
	policyDigest := hex.EncodeToString(sum[:])[:16]

	action, _ := flagValue(args, "action")
	if action == "" {
		fmt.Fprintln(os.Stderr, "--action is required: a decision with no action is not a decision")
		return 2
	}

	facts := policy.Facts{
		Action:            action,
		Actor:             flagValueOr(args, "actor", ""),
		Risk:              flagValueOr(args, "risk", "R0"),
		Reversibility:     flagValueOr(args, "reversibility", "revert"),
		Confidence:        flagValueOr(args, "confidence", "unknown"),
		Observability:     flagValueOr(args, "observability", "silent"),
		RollbackRehearsed: hasFlag(args, "rollback-rehearsed"),
	}
	requested := flagValueOr(args, "requested", "A2")

	decision := policy.Evaluate(p, policyDigest, facts, requested)

	if hasFlag(args, "json") {
		b, _ := json.MarshalIndent(decision, "", "  ")
		fmt.Println(string(b))
	} else {
		mark := map[string]string{"allow": "ALLOW", "refuse": "REFUSE", "escalate": "ESCALATE"}[decision.Verdict]
		fmt.Printf("%s  %s\n\n", mark, facts.Action)
		fmt.Printf("  risk %s · reversibility %s · confidence %s · observability %s\n",
			facts.Risk, facts.Reversibility, facts.Confidence, facts.Observability)
		fmt.Printf("  ceiling %s · requested %s · policy %s\n\n", decision.Dimensions.Autonomy, requested, policyDigest)
		for _, r := range decision.Reasons {
			fmt.Printf("  · %s\n", r)
		}
	}

	return policy.ExitCode(decision.Verdict)
}

func flagValueOr(args []string, name, def string) string {
	if v, ok := flagValue(args, name); ok {
		return v
	}
	return def
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name {
			return true
		}
	}
	return false
}
