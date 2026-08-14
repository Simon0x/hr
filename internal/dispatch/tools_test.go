package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, department, frontmatter string) {
	t.Helper()
	dir := filepath.Join(root, "skills", strings.ToLower(department))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" + frontmatter + "\n---\n\nprocedure body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The bug this pins: truncating an entry at '(' turns a grant to two exact
// commands into a grant to the whole shell.
func TestGrantForDepartment_KeepsArgumentPatterns(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "QA", "allowed-tools: Read, Grep, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr impact *)")

	grant := GrantForDepartment(root, "QA")

	for _, spec := range grant.Allow {
		if spec == "Bash" {
			t.Fatalf("a scoped Bash grant collapsed to bare %q - that is the whole shell, not the two commands granted: %v", spec, grant.Allow)
		}
	}
	want := []string{
		"Read", "Grep",
		"Bash(cd " + root + " && go build -o hr ./cmd/hr)",
		"Bash(" + root + "/hr impact *)",
	}
	if strings.Join(grant.Allow, "|") != strings.Join(want, "|") {
		t.Errorf("grant.Allow =\n  %v\nwant\n  %v", grant.Allow, want)
	}
}

func TestGrantForDepartment_ParsesTheDenyList(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "QA", "disallowed-tools: Edit, Write, NotebookEdit\nallowed-tools: Read, Grep")

	grant := GrantForDepartment(root, "QA")

	if strings.Join(grant.Deny, ",") != "Edit,Write,NotebookEdit" {
		t.Errorf("grant.Deny = %v, want [Edit Write NotebookEdit] - a declared denial that is never passed is not a denial", grant.Deny)
	}
	if strings.Join(grant.Allow, ",") != "Read,Grep" {
		t.Errorf("grant.Allow = %v", grant.Allow)
	}
}

func TestGrantForDepartment_UnconfiguredSeatIsReadOnly(t *testing.T) {
	root := t.TempDir()

	grant := GrantForDepartment(root, "Nonexistent")

	if strings.Join(grant.Allow, ",") != strings.Join(DefaultAllowedTools, ",") {
		t.Errorf("grant.Allow = %v, want the read-only default %v", grant.Allow, DefaultAllowedTools)
	}
	for _, spec := range grant.Allow {
		if spec == "Bash" || spec == "Write" || spec == "Edit" {
			t.Errorf("default grant includes %q - an unconfigured seat must not be able to act", spec)
		}
	}
}

// The real skills ship scoped grants; hr must carry them through rather than
// widen them.
func TestGrantForDepartment_RealSkillsAreScoped(t *testing.T) {
	root := findRepoRoot(t)

	for _, dept := range []string{"QA", "Engineering"} {
		grant := GrantForDepartment(root, dept)
		var bareBash bool
		var scopedBash int
		for _, spec := range grant.Allow {
			if spec == "Bash" {
				bareBash = true
			}
			if strings.HasPrefix(spec, "Bash(") {
				scopedBash++
			}
		}
		if bareBash {
			t.Errorf("%s: grant includes bare Bash: %v", dept, grant.Allow)
		}
		if scopedBash == 0 {
			t.Errorf("%s: no scoped Bash grant parsed from the real SKILL.md: %v", dept, grant.Allow)
		}
		if len(grant.Deny) == 0 {
			t.Errorf("%s: SKILL.md declares disallowed-tools but the grant carries none", dept)
		}
	}
}
