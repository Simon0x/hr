package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/guard"
	"github.com/Simon0x/hr/internal/policy"
)

// Exit codes are the PreToolUse hook contract: 0 lets the call through, 2
// blocks it and shows stderr to the model. Anything else is a hook failure,
// which must not become permission - see failClosed.
const (
	guardAllow = 0
	guardDeny  = 2
)

// @gate validate:ledger
// @covers internal/guard/**
// @covers cmd/hr/guard.go
//
// cmdGuard is the policy engine at the point of action. A harness runs it
// before each tool call, hands it the call on stdin, and blocks the call if
// it exits 2. Everything it needs about the step comes from the environment
// the dispatching process set.
func cmdGuard(root string, _ []string) int {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return failClosed("reading the tool call: %v", err)
	}
	call, err := guard.ParseCall(raw)
	if err != nil {
		return failClosed("%v", err)
	}

	p, rawPolicy, err := policy.LoadPolicy(filepath.Join(root, "policies", "default.json"))
	if err != nil {
		return failClosed("loading policy: %v", err)
	}
	sum := sha256.Sum256(rawPolicy)
	digest := hex.EncodeToString(sum[:])[:16]

	ctx := guard.Context{
		Actor:         os.Getenv("HR_GUARD_ACTOR"),
		Goal:          os.Getenv("HR_GUARD_GOAL"),
		Department:    os.Getenv("HR_GUARD_DEPARTMENT"),
		Confidence:    os.Getenv("HR_GUARD_CONFIDENCE"),
		Observability: os.Getenv("HR_GUARD_OBSERVABILITY"),
		Requested:     os.Getenv("HR_GUARD_AUTONOMY"),
	}

	decision, shape, allowed := guard.Decide(p, digest, ctx, call)
	if allowed {
		return guardAllow
	}

	if token := os.Getenv("HR_GUARD_TOKEN"); token != "" {
		if err := guard.RecordDenial(root, token, guard.Denial{
			Action: guard.Describe(call), Tool: call.ToolName,
			Risk: shape.Risk, Rev: shape.Reversibility, Why: shape.Why,
			Verdict: decision.Verdict, Policy: decision.Policy, Reasons: decision.Reasons,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "hr guard: recording the refusal failed: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stderr,
		"hr policy refused this call: %s scored %s/%s (%s), verdict %s under policy %s.\n"+
			"This is a policy decision, not a tool error - do not retry it or route around it. "+
			"Report it and stop.\n",
		call.ToolName, shape.Risk, shape.Reversibility, shape.Why, decision.Verdict, decision.Policy)
	for _, r := range decision.Reasons {
		fmt.Fprintf(os.Stderr, "  - %s\n", r)
	}
	return guardDeny
}

// failClosed denies when the guard cannot reach a decision. A control that
// waves work through when it breaks is not a control; the cost of getting
// this backwards is that every unreadable policy file becomes permission.
func failClosed(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "hr guard could not decide, so it refused: "+format+"\n", args...)
	return guardDeny
}
