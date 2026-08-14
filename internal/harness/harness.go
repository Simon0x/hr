package harness

import (
	"context"
	"encoding/json"
	"errors"
)

type Result struct {
	OK         bool
	ExitCode   int
	Output     string
	CostUSD    float64
	Tokens     float64
	DurationMS int64
	NumTurns   int
	SessionID  string
}

type StructuredResult struct {
	Result
	StructuredOutput json.RawMessage
}

// Grant is the tool authority one invocation runs under. Allow entries may
// carry an argument pattern (`Bash(hr impact *)`); Deny always wins over
// Allow. A harness that cannot enforce Deny must refuse the invocation
// rather than run it wider than asked - silently dropping the deny half is
// how a seat ends up with the whole shell.
type Grant struct {
	Allow []string
	Deny  []string
}

func (g Grant) IsZero() bool { return len(g.Allow) == 0 && len(g.Deny) == 0 }

// ErrGrantUnenforceable is returned when a harness is handed authority it
// cannot hold to. Callers treat it as a blocked step, not as evidence about
// the work: nothing ran.
var ErrGrantUnenforceable = errors.New("harness cannot enforce the declared grant")

type Harness interface {
	// Name is the HR_HARNESS selector name, recorded against every
	// invocation so a chain spanning a harness switch still says which
	// agent produced which work.
	Name() string
	// CheckGrant reports whether this harness can enforce grant as written.
	// Returning an error is how a harness refuses work it could only run
	// wider than asked; Select wraps every harness so the refusal happens
	// whether or not a call site remembers to ask.
	CheckGrant(g Grant) error
	Invoke(ctx context.Context, root, prompt string, grant Grant) (Result, error)
	InvokeStructured(ctx context.Context, root, prompt string, schema []byte, grant Grant) (StructuredResult, error)
}
