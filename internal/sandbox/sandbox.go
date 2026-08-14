// Package sandbox is the Service Definition for confining a process hr is
// about to spawn.
//
// Everything above it decides what an agent may do: the policy engine judges
// each tool call, the tool grant names the commands a seat holds. Both are
// enforced by something hr does not control - the harness honouring a hook,
// a CLI honouring its own matcher. This is the layer that does not depend on
// the agent, or the harness, cooperating.
package sandbox

import "os/exec"

// Policy is what a confined process may change. Reads and execution are
// deliberately not restricted: the confinement that matters for a seat whose
// denials are `Edit` and `Write` is write confinement, and restricting reads
// breaks a toolchain long before it stops anything.
type Policy struct {
	// Writable are the only roots the process may create, modify or delete
	// under. Everything else on the filesystem becomes read-only.
	Writable []string
}

// Sandbox wraps the argv of a process before it is spawned.
type Sandbox interface {
	// Name is what the ledger records as the confinement in force.
	Name() string
	// Wrap returns the argv to spawn in place of argv. A provider that
	// cannot confine returns argv unchanged and says so through Name, so
	// nothing reads as confined when it is not.
	Wrap(argv []string, p Policy) ([]string, error)
}

// None is the honest absence of confinement, for platforms with no backend.
// It exists so a deployment without a sandbox is a stated fact rather than a
// silently missing control.
type None struct{}

var _ Sandbox = None{}

func (None) Name() string { return "none" }

func (None) Wrap(argv []string, _ Policy) ([]string, error) { return argv, nil }

// Available reports whether a real backend can confine on this host.
func Available() bool { return available() }

// Select returns the strongest backend this host supports.
func Select() Sandbox {
	if available() {
		return Landlock{}
	}
	return None{}
}

// Apply confines the calling process. It is called by the helper the
// Landlock backend wraps argv with, immediately before exec.
func Apply(p Policy) error { return apply(p) }

// ensure exec stays referenced for platforms whose apply is a stub.
var _ = exec.Command
