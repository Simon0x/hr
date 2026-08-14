package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/ledger"
)

// loadAgentDef reads a Claude Code subagent definition from agents/<name>.md
// and returns the tool grant and instructions it declares, so any Harness
// can be invoked as that agent via a plain Invoke call rather than requiring
// a CLI-native named-subagent mechanism. ${CLAUDE_PLUGIN_ROOT} in the body
// is resolved to root, the same directory agents/ was read from - Claude
// Code's own plugin loader would otherwise do this substitution implicitly.
func agentDefPath(name string) string { return filepath.Join("agents", name+".md") }

func loadAgentDef(root, name string) (grant harness.Grant, instructions string, err error) {
	path := filepath.Join(root, agentDefPath(name))
	raw, err := os.ReadFile(path)
	if err != nil {
		return harness.Grant{}, "", fmt.Errorf("loading agent %q: %w", name, err)
	}

	lines := strings.Split(string(raw), "\n")
	inFrontmatter := false
	bodyStart := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontmatter {
				bodyStart = i + 1
				break
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "tools:"); ok {
			grant.Allow = toolSpecs(rest, root)
		}
	}

	body := strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	body = strings.ReplaceAll(body, "${CLAUDE_PLUGIN_ROOT}", root)
	return grant, body, nil
}

// invokeAgent runs a named agent definition against any Harness: load its
// tool grant and instructions, then a plain Invoke with the task appended.
// A load failure is returned as the invocation's own error rather than
// surfaced separately, matching how any other Invoke failure is reported.
// The returned Request is what the ledger records for this invocation.
func invokeAgent(ctx context.Context, root, name, task string, h harness.Harness) (harness.Result, *ledger.Request, error) {
	grant, instructions, err := loadAgentDef(root, name)
	if err != nil {
		return harness.Result{}, nil, err
	}
	prompt := instructions + "\n\n" + task
	req := NewRequest(root, h, prompt, grant, agentDefPath(name))
	result, err := h.Invoke(ctx, root, prompt, grant)
	return result, req, err
}
