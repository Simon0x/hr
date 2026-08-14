package ledger

import (
	"encoding/json"
	"testing"
)

// A line written before Entry gained Request must re-canonicalize to exactly
// the bytes it was written as. Digest() re-marshals the struct, so any field
// that serialized differently after the change would alter every prev hash
// after it and break the chain retroactively.
func TestCanonicalIsStableForEntriesWrittenBeforeRequest(t *testing.T) {
	lines := []string{
		`{"seq":0,"prev":"0000000000000000000000000000000000000000000000000000000000000000","at":"2026-08-12T10:00:00.000Z","kind":"decision","actor":"spiffe://hr/dispatch","outcome":"ok","detail":"Release: allow"}`,
		`{"seq":1,"prev":"abc","at":"2026-08-12T10:00:01.000Z","kind":"action","actor":"spiffe://hr/dispatch","artifacts":{"in":[],"out":["a1"]},"goal":"deadbeef1234","outcome":"ok","detail":"ran","cost":{"tokens":10,"seconds":1.5,"currency":"USD","amount":0.02}}`,
	}

	for i, line := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if e.Request != nil {
			t.Fatalf("line %d: Request should be absent, got %+v", i, e.Request)
		}
		got, err := Canonical(e)
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if string(got) != line {
			t.Errorf("line %d canonical drift:\n want %s\n  got %s", i, line, got)
		}
	}
}

func TestChainVerifiesAcrossEntriesWithAndWithoutRequest(t *testing.T) {
	entries := []Entry{
		{Kind: "decision", Actor: "a", At: "2026-08-12T10:00:00.000Z", Outcome: "ok"},
		{Kind: "action", Actor: "a", At: "2026-08-12T10:00:01.000Z", Outcome: "ok", Request: &Request{
			Harness: "claude", PromptDigest: TextDigest("do the thing"), Tools: []string{"Read", "Grep"},
			Procedure: "departments/engineering.md", ProcedureDigest: TextDigest("# Engineering"),
		}},
		{Kind: "action", Actor: "a", At: "2026-08-12T10:00:02.000Z", Outcome: "ok"},
	}

	expected := Genesis
	for i := range entries {
		entries[i].Seq = i
		entries[i].Prev = expected
		d, err := Digest(entries[i])
		if err != nil {
			t.Fatal(err)
		}
		expected = d
	}

	if msg, ok := VerifyChain(entries); !ok {
		t.Fatalf("chain should verify: %s", msg)
	}
}

func TestRequestSurvivesRoundTrip(t *testing.T) {
	in := Entry{Seq: 0, Prev: Genesis, At: "2026-08-12T10:00:00.000Z", Kind: "action", Actor: "a",
		Request: &Request{Harness: "codex", PromptDigest: TextDigest("p"), Tools: []string{}}}

	raw, err := Canonical(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Entry
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Request == nil {
		t.Fatal("request lost")
	}
	if out.Request.Harness != "codex" || out.Request.PromptDigest != TextDigest("p") {
		t.Errorf("request altered: %+v", out.Request)
	}
	if out.Request.Tools == nil || len(out.Request.Tools) != 0 {
		t.Errorf("empty grant should stay an empty array, got %#v", out.Request.Tools)
	}
	if d1, _ := Digest(in); d1 != mustDigest(t, out) {
		t.Error("digest changed across round trip")
	}
}

func mustDigest(t *testing.T, e Entry) string {
	t.Helper()
	d, err := Digest(e)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
