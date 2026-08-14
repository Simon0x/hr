// Package guard evaluates a single tool call against the policy engine, at
// the moment the agent tries to make it.
//
// The engine was already consulted once per step, before the agent started,
// and then had no say in anything the agent did. This is the same engine at
// the point of action: what a step is allowed to attempt, judged per attempt
// rather than per step.
package guard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Simon0x/hr/internal/policy"
)

// Call is the PreToolUse payload a harness hands to the guard. Field names
// follow Claude Code's hook wire format; a second harness maps its own shape
// onto this rather than the guard learning a second dialect.
type Call struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	ToolUseID string         `json:"tool_use_id"`
	SessionID string         `json:"session_id"`
	CWD       string         `json:"cwd"`
}

// Context is what the dispatching step knew when it authorised the step, so
// a tool call is judged in the situation it belongs to rather than in
// isolation. It reaches the guard process through the environment.
type Context struct {
	Actor         string
	Goal          string
	Department    string
	Confidence    string
	Observability string
	Requested     string
	// Token names this invocation's denial log. Empty means refusals are
	// still enforced but not recorded.
	Token string
	// Root is the project the guard evaluates in; the hook process starts
	// in the agent's cwd and needs to be told.
	Root string
}

// Shape is the risk and reversibility a tool call carries, independent of
// the step that produced it.
type Shape struct {
	Risk          string
	Reversibility string
	Why           string
}

// destructive matches shell commands whose effect cannot be walked back by
// re-running anything: the point is not to enumerate every bad command, but
// to stop the ones whose reversibility claim would be a lie.
var destructive = []struct {
	pattern string
	why     string
}{
	{"rm -rf", "recursive delete"},
	{"rm -fr", "recursive delete"},
	{"git push --force", "force push rewrites published history"},
	{"git push -f", "force push rewrites published history"},
	{"git reset --hard", "discards working tree state"},
	{"drop table", "destructive SQL"},
	{"drop database", "destructive SQL"},
	{"truncate table", "destructive SQL"},
	{"mkfs", "filesystem creation"},
	{"dd if=", "raw device write"},
	{"chmod -r 777", "recursive permission widening"},
	{"curl", "unreviewed network fetch"},
	{"wget", "unreviewed network fetch"},
}

// readOnly names tools that observe and never change anything.
var readOnly = map[string]bool{
	"Read": true, "Grep": true, "Glob": true, "NotebookRead": true,
	"WebSearch": true, "WebFetch": true, "TodoWrite": true,
}

// mutating names tools whose effect is a file change a revert undoes.
var mutating = map[string]bool{
	"Write": true, "Edit": true, "MultiEdit": true, "NotebookEdit": true,
}

// ShapeOf scores one call. An unknown tool is treated as mutating rather
// than read-only: a tool nobody has classified is not evidence that it is
// harmless.
func ShapeOf(c Call) Shape {
	switch {
	case readOnly[c.ToolName]:
		return Shape{Risk: "R0", Reversibility: "reversible", Why: "read-only tool"}
	case mutating[c.ToolName]:
		return Shape{Risk: "R1", Reversibility: "revert", Why: "file mutation"}
	case c.ToolName == "Bash" || c.ToolName == "pwsh":
		cmd := strings.ToLower(commandOf(c))
		for _, d := range destructive {
			if strings.Contains(cmd, d.pattern) {
				return Shape{Risk: "R3", Reversibility: "irreversible", Why: d.why}
			}
		}
		return Shape{Risk: "R2", Reversibility: "revert", Why: "shell command"}
	default:
		return Shape{Risk: "R1", Reversibility: "revert", Why: "unclassified tool, scored as mutating"}
	}
}

func commandOf(c Call) string {
	if v, ok := c.ToolInput["command"].(string); ok {
		return v
	}
	return ""
}

// Describe renders the action string the policy engine and the ledger see.
// Tool input is summarised, never echoed whole: an argument can be arbitrary
// text a model was handed, and a decision record is not the place for it.
func Describe(c Call) string {
	if cmd := commandOf(c); cmd != "" {
		return fmt.Sprintf("%s: %s", c.ToolName, truncate(cmd, 120))
	}
	keys := make([]string, 0, len(c.ToolInput))
	for k := range c.ToolInput {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return c.ToolName
	}
	return fmt.Sprintf("%s(%s)", c.ToolName, strings.Join(keys, ", "))
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Decide evaluates one call. An escalate verdict denies: there is nobody to
// escalate to inside a running invocation, and treating "a human should look
// at this" as permission to proceed is the failure the engine exists to stop.
func Decide(p *policy.Policy, policyDigest string, ctx Context, c Call) (policy.Decision, Shape, bool) {
	shape := ShapeOf(c)
	requested := ctx.Requested
	if requested == "" {
		requested = "A2"
	}
	decision := policy.Evaluate(p, policyDigest, policy.Facts{
		Action:        Describe(c),
		Actor:         ctx.Actor,
		Risk:          shape.Risk,
		Reversibility: shape.Reversibility,
		Confidence:    orDefault(ctx.Confidence, "likely"),
		Observability: orDefault(ctx.Observability, "logs"),
	}, requested)
	return decision, shape, decision.Verdict == "allow"
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// ParseCall reads a PreToolUse payload.
func ParseCall(raw []byte) (Call, error) {
	var c Call
	if err := json.Unmarshal(raw, &c); err != nil {
		return Call{}, fmt.Errorf("not a tool-call payload: %w", err)
	}
	if c.ToolName == "" {
		return Call{}, fmt.Errorf("payload names no tool")
	}
	return c, nil
}
