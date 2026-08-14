//go:build !linux

package sandbox

import "fmt"

// Landlock is a Linux LSM; elsewhere it is unavailable rather than absent
// from the vocabulary, so a build on another platform still names the seam.
type Landlock struct{}

var _ Sandbox = Landlock{}

func (Landlock) Name() string { return "landlock" }

func (Landlock) Wrap(argv []string, _ Policy) ([]string, error) { return argv, nil }

func available() bool { return false }

func apply(Policy) error { return fmt.Errorf("landlock is Linux-only") }
