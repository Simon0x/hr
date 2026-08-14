package harness

import (
	"strings"
	"testing"
)

// Each entry must be its own argv value. An argument pattern contains spaces
// and may contain a comma, so a comma-joined string would split entries in
// the wrong places and the CLI would receive a grant nobody wrote.
func TestGrantArgs_PassesEachEntryAsItsOwnArgument(t *testing.T) {
	grant := Grant{
		Allow: []string{"Read", "Bash(cd /repo && go build -o hr ./cmd/hr)"},
		Deny:  []string{"Edit", "Write"},
	}

	args := grantArgs(grant)

	joined := strings.Join(args, "\x00")
	if !strings.Contains(joined, "Bash(cd /repo && go build -o hr ./cmd/hr)") {
		t.Fatalf("pattern did not survive as one argument: %#v", args)
	}
	allowAt, denyAt := -1, -1
	for i, a := range args {
		switch a {
		case "--allowedTools":
			allowAt = i
		case "--disallowedTools":
			denyAt = i
		}
	}
	if allowAt < 0 || denyAt < 0 {
		t.Fatalf("both flags must be present: %#v", args)
	}
	if got := args[denyAt+1 : denyAt+3]; got[0] != "Edit" || got[1] != "Write" {
		t.Errorf("deny entries = %v, want [Edit Write]", got)
	}
	if args[allowAt+2] != "Bash(cd /repo && go build -o hr ./cmd/hr)" {
		t.Errorf("allow entries misplaced: %#v", args)
	}
}

func TestGrantArgs_OmitsFlagsItHasNothingFor(t *testing.T) {
	args := grantArgs(Grant{Allow: []string{"Read"}})
	for _, a := range args {
		if a == "--disallowedTools" {
			t.Fatalf("emitted an empty --disallowedTools: %#v", args)
		}
	}
	if len(grantArgs(Grant{})) != 0 {
		t.Errorf("an empty grant should add no flags, got %#v", grantArgs(Grant{}))
	}
}
