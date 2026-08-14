package harness

import (
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/sandbox"
)

// SandboxName reports the confinement a spawned agent will run under, for
// the record and for anyone asking whether there is one.
func SandboxName() string { return sandbox.Select().Name() }

// confine wraps the argv hr is about to spawn so the agent cannot write
// outside the places its work belongs in. It is the layer beneath the tool
// grant and the policy guard: those need the harness to cooperate, this does
// not.
//
// HR_SANDBOX=off disables it. The escape exists because a confinement that
// cannot be turned off gets worked around in ways nobody records, and an
// operator saying so is better than an operator improvising.
func confine(root string, argv []string) ([]string, error) {
	if os.Getenv("HR_SANDBOX") == "off" {
		return argv, nil
	}
	return sandbox.Select().Wrap(argv, sandbox.Policy{Writable: writableRoots(root)})
}

// writableRoots is where an agent legitimately writes: the project it was
// pointed at, the scratch space every toolchain assumes, the device nodes a
// shell needs, and the agent product's own state directory. Everything else
// on the filesystem becomes read-only.
func writableRoots(root string) []string {
	roots := []string{root, "/tmp", "/dev", "/var/tmp", "/run"}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		roots = append(roots, tmp)
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots,
			filepath.Join(home, ".claude"),
			filepath.Join(home, ".cache"),
			filepath.Join(home, ".config"),
			filepath.Join(home, ".npm"),
		)
	}
	return roots
}
