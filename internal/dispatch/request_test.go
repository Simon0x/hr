package dispatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/ledger"
)

func TestNewRequestPinsPromptAndProcedure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "departments"), 0o755); err != nil {
		t.Fatal(err)
	}
	proc := filepath.Join("departments", "engineering.md")
	if err := os.WriteFile(filepath.Join(root, proc), []byte("# Engineering\nstep one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := NewRequest(root, &harness.Mock{}, "the prompt", harness.Grant{Allow: []string{"Read", "Bash"}}, proc)

	if req.Harness != "mock" {
		t.Errorf("harness = %q, want mock", req.Harness)
	}
	if req.PromptDigest != ledger.TextDigest("the prompt") {
		t.Error("prompt digest does not match the prompt that ran")
	}
	if req.ProcedureDigest != ledger.TextDigest("# Engineering\nstep one\n") {
		t.Error("procedure digest does not match the procedure on disk")
	}

	// Editing the procedure must change the digest a later entry records,
	// which is the point: a past entry keeps meaning what it meant.
	if err := os.WriteFile(filepath.Join(root, proc), []byte("# Engineering\nstep two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := NewRequest(root, &harness.Mock{}, "the prompt", harness.Grant{Allow: []string{"Read", "Bash"}}, proc)
	if after.ProcedureDigest == req.ProcedureDigest {
		t.Error("procedure digest did not change after the procedure was edited")
	}
}

func TestNewRequestNormalizesAbsentGrantAndProcedure(t *testing.T) {
	root := t.TempDir()

	req := NewRequest(root, &harness.Mock{MockName: "codex"}, "p", harness.Grant{}, "")

	if req.Harness != "codex" {
		t.Errorf("harness = %q, want codex", req.Harness)
	}
	if req.Tools == nil || len(req.Tools) != 0 {
		t.Errorf("nil grant should record as an empty array, got %#v", req.Tools)
	}
	if req.Procedure != "" || req.ProcedureDigest != "" {
		t.Errorf("no procedure should leave both fields empty, got %+v", req)
	}
}

// An unreadable procedure must not fail the invocation; it records less.
func TestNewRequestToleratesMissingProcedure(t *testing.T) {
	req := NewRequest(t.TempDir(), &harness.Mock{}, "p", harness.Grant{Allow: []string{"Read"}}, "departments/gone.md")

	if req.Procedure != "departments/gone.md" {
		t.Errorf("procedure path should still be recorded, got %q", req.Procedure)
	}
	if req.ProcedureDigest != "" {
		t.Errorf("missing procedure should leave the digest empty, got %q", req.ProcedureDigest)
	}
}
