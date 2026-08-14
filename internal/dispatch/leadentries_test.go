package dispatch

import (
	"testing"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/ledger"
)

func TestLeadEntriesRecordEveryLeadInvocation(t *testing.T) {
	root := t.TempDir()
	outcome := DiscoveryOutcome{Leads: []LeadResult{
		{Lead: Lead{Name: "structural", Prompt: "check structure", Grant: harness.Grant{Allow: []string{"Read"}}},
			Result: harness.Result{OK: true, CostUSD: 0.01, Tokens: 100, DurationMS: 500}},
		{Lead: Lead{Name: "mechanism", Prompt: "check mechanism", Grant: harness.Grant{Allow: []string{"Read", "Grep"}}},
			Result: harness.Result{OK: false}, Blocked: true},
	}}

	entries := LeadEntries(root, &harness.Mock{}, outcome, "spiffe://hr/w", "goal123", "departments/discovery.md")

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want one per lead", len(entries))
	}
	for i, e := range entries {
		if e.Kind != "action" {
			t.Errorf("entry %d kind = %q, want action", i, e.Kind)
		}
		if e.Request == nil {
			t.Fatalf("entry %d records no request", i)
		}
		if e.Goal != "goal123" {
			t.Errorf("entry %d goal = %q", i, e.Goal)
		}
		if e.Request.PromptDigest != ledger.TextDigest(outcome.Leads[i].Lead.Prompt) {
			t.Errorf("entry %d prompt digest does not match the lead's prompt", i)
		}
	}
	if entries[0].Outcome != "ok" {
		t.Errorf("settled lead outcome = %q, want ok", entries[0].Outcome)
	}
	if entries[1].Outcome != "blocked" {
		t.Errorf("blocked lead outcome = %q, want blocked", entries[1].Outcome)
	}
	if entries[0].Cost == nil {
		t.Error("a lead that cost something recorded no cost")
	}
}
