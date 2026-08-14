package harness

import (
	"context"
	"errors"
	"testing"
)

// The constraint is that a harness which cannot enforce a grant refuses the
// work rather than running it wider than asked. Select wraps every harness
// so that holds without each call site remembering to ask.
func TestSelect_RefusesAnInvocationItCannotEnforce(t *testing.T) {
	h, err := Select("mock")
	if err != nil {
		t.Fatal(err)
	}
	mock := inner(h).(*Mock)
	mock.GrantErr = errors.New("no deny-list support")

	grant := Grant{Allow: []string{"Read"}, Deny: []string{"Write"}}

	if _, err := h.Invoke(context.Background(), t.TempDir(), "p", grant); !errors.Is(err, ErrGrantUnenforceable) {
		t.Errorf("Invoke error = %v, want ErrGrantUnenforceable", err)
	}
	if _, err := h.InvokeStructured(context.Background(), t.TempDir(), "p", nil, grant); !errors.Is(err, ErrGrantUnenforceable) {
		t.Errorf("InvokeStructured error = %v, want ErrGrantUnenforceable", err)
	}
	if len(mock.Calls) != 0 {
		t.Errorf("the harness ran %d time(s) after refusing the grant - nothing must reach it", len(mock.Calls))
	}
}

func TestSelect_RunsWhenTheGrantIsEnforceable(t *testing.T) {
	h, err := Select("mock")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Invoke(context.Background(), t.TempDir(), "p", Grant{Allow: []string{"Read"}}); err != nil {
		t.Fatalf("an enforceable grant must run: %v", err)
	}
	if inner(h).(*Mock).Calls[0].Grant.Allow[0] != "Read" {
		t.Error("the grant did not reach the harness intact")
	}
}

func TestClaude_EnforcesBothHalvesOfAGrant(t *testing.T) {
	if err := (Claude{}).CheckGrant(Grant{Allow: []string{"Read"}, Deny: []string{"Write"}}); err != nil {
		t.Errorf("Claude maps both halves to CLI flags, so it must accept: %v", err)
	}
}
