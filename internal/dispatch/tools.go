package dispatch

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/Simon0x/hr/internal/harness"
)

// DefaultAllowedTools is the tool grant a department falls back to when its
// SKILL.md declares none - read-only, so an unconfigured seat can look
// around but never act.
var DefaultAllowedTools = []string{"Read", "Grep", "Glob"}

// GrantForDepartment reads the seat's declared grant from its SKILL.md
// frontmatter. Entries keep any parenthesised argument pattern intact:
// `Bash(hr impact *)` is a grant to one command, and truncating it to `Bash`
// would hand over the whole shell.
func GrantForDepartment(root, department string) harness.Grant {
	path := filepath.Join(root, "skills", strings.ToLower(department), "SKILL.md")
	f, err := os.Open(path)
	if err != nil {
		return harness.Grant{Allow: DefaultAllowedTools}
	}
	defer f.Close()

	var grant harness.Grant
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "disallowed-tools:"); ok {
			grant.Deny = toolSpecs(rest, root)
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "allowed-tools:"); ok {
			grant.Allow = toolSpecs(rest, root)
		}
	}
	if len(grant.Allow) == 0 {
		grant.Allow = DefaultAllowedTools
	}
	return grant
}

// toolSpecs splits a frontmatter tool list on its top-level commas, so a
// comma inside an argument pattern does not split the entry it belongs to,
// and resolves the path variables a SKILL.md may use.
func toolSpecs(raw, root string) []string {
	var specs []string
	seen := map[string]bool{}
	depth := 0
	start := 0
	flush := func(end int) {
		spec := strings.TrimSpace(raw[start:end])
		spec = strings.ReplaceAll(spec, "${CLAUDE_PROJECT_DIR}", root)
		spec = strings.ReplaceAll(spec, "${CLAUDE_PLUGIN_ROOT}", root)
		if spec != "" && !seen[spec] {
			seen[spec] = true
			specs = append(specs, spec)
		}
	}
	for i, r := range raw {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				flush(i)
				start = i + 1
			}
		}
	}
	flush(len(raw))
	return specs
}
