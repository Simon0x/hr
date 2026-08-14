package hrserver

import (
	"strings"
	"testing"

	"github.com/Simon0x/hr/internal/ledger"
)

func invocationEntry() ledger.Entry {
	return ledger.Entry{Kind: "action", Actor: "spiffe://hr/worker", Outcome: "ok", Detail: `lead "mechanism"`,
		Request: &ledger.Request{Harness: "claude", PromptDigest: ledger.TextDigest("p"), Tools: []string{"Read"}}}
}

func TestCheckPriorEntriesAcceptsInvocationRecords(t *testing.T) {
	if problem := checkPriorEntries([]ledger.Entry{invocationEntry(), invocationEntry()}); problem != "" {
		t.Fatalf("valid prior entries rejected: %s", problem)
	}
	if problem := checkPriorEntries(nil); problem != "" {
		t.Fatalf("absent prior rejected: %s", problem)
	}
}

// prior must not become a way to write arbitrary ledger entries behind a
// valid lease - a forged decision is the case that matters.
func TestCheckPriorEntriesRejectsAnythingButInvocations(t *testing.T) {
	forgedDecision := invocationEntry()
	forgedDecision.Kind = "decision"

	noRequest := invocationEntry()
	noRequest.Request = nil

	withArtifacts := invocationEntry()
	withArtifacts.Artifacts = &ledger.Artifacts{In: []string{}, Out: []string{"a1"}}

	cases := map[string]struct {
		entry ledger.Entry
		want  string
	}{
		"forged decision":  {forgedDecision, "only \"action\" is accepted"},
		"no request":       {noRequest, "no request"},
		"claims artifacts": {withArtifacts, "do not carry artifacts"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			problem := checkPriorEntries([]ledger.Entry{tc.entry})
			if problem == "" {
				t.Fatal("accepted, want rejected")
			}
			if !strings.Contains(problem, tc.want) {
				t.Errorf("problem = %q, want it to mention %q", problem, tc.want)
			}
		})
	}
}

func TestCheckPriorEntriesIsBounded(t *testing.T) {
	tooMany := make([]ledger.Entry, maxPriorEntries+1)
	for i := range tooMany {
		tooMany[i] = invocationEntry()
	}
	if problem := checkPriorEntries(tooMany); !strings.Contains(problem, "too many prior entries") {
		t.Fatalf("unbounded prior accepted: %q", problem)
	}
}
