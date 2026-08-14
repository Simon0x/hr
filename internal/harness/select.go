package harness

import (
	"context"
	"fmt"
)

// Select resolves a harness by name, following the HR_HARNESS env var
// convention every other cross-cutting setting (HR_ACTOR, HR_TOKEN, ...)
// already uses. An empty name defaults to Claude, hr's only real harness
// today; "mock" selects a harness that succeeds without invoking any CLI,
// useful for exercising the dispatch path without an agent installed.
func Select(name string) (Harness, error) {
	switch name {
	case "", "claude":
		return guarded{Claude{}}, nil
	case "mock":
		return guarded{&Mock{Result: Result{OK: true, Output: "mock harness: no real agent invoked"}}}, nil
	default:
		return nil, fmt.Errorf("unknown harness %q (HR_HARNESS wants claude or mock)", name)
	}
}

// guarded refuses an invocation whose grant its harness cannot enforce.
// Select wraps every harness in it, so the check happens once at the seam
// rather than at each of the call sites that must not forget it.
type guarded struct{ Harness }

func (g guarded) Invoke(ctx context.Context, root, prompt string, grant Grant) (Result, error) {
	if err := g.CheckGrant(grant); err != nil {
		return Result{}, fmt.Errorf("%w: %s: %w", ErrGrantUnenforceable, g.Name(), err)
	}
	return g.Harness.Invoke(ctx, root, prompt, grant)
}

func (g guarded) InvokeStructured(ctx context.Context, root, prompt string, schema []byte, grant Grant) (StructuredResult, error) {
	if err := g.CheckGrant(grant); err != nil {
		return StructuredResult{}, fmt.Errorf("%w: %s: %w", ErrGrantUnenforceable, g.Name(), err)
	}
	return g.Harness.InvokeStructured(ctx, root, prompt, schema, grant)
}
