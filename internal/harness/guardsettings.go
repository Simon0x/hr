package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/guard"
)

// guardSettings writes the settings file that puts hr's policy engine in
// front of every tool call, and returns the extra argv and environment the
// child needs. An empty path means no guard context was attached, so the
// invocation runs as it did before.
//
// The hook is unmatched deliberately: a matcher would decide which tools are
// worth judging, which is the policy engine's job, not the wiring's.
func guardSettings(ctx context.Context, dir string) (args []string, env []string, err error) {
	gc, ok := guard.FromContext(ctx)
	if !ok {
		return nil, nil, nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}

	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": self + " guard"},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, "hr-guard-settings.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, nil, err
	}

	return []string{"--settings", path}, []string{
		"HR_GUARD_ACTOR=" + gc.Actor,
		"HR_GUARD_GOAL=" + gc.Goal,
		"HR_GUARD_DEPARTMENT=" + gc.Department,
		"HR_GUARD_CONFIDENCE=" + gc.Confidence,
		"HR_GUARD_OBSERVABILITY=" + gc.Observability,
		"HR_GUARD_AUTONOMY=" + gc.Requested,
		"HR_GUARD_TOKEN=" + gc.Token,
		"HR_ROOT=" + gc.Root,
	}, nil
}
