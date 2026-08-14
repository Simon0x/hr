package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/Simon0x/hr/internal/sandbox"
)

// @gate validate:ledger
// @covers internal/sandbox/**
// @covers cmd/hr/confine.go
//
// cmdConfine restricts this process and then becomes the command it was
// asked to run. It exists because a Landlock ruleset applies to the calling
// process and Go offers no hook between fork and exec, so the only way to
// hand a confined process to exec is to be that process first.
//
// Usage: hr confine [--writable DIR]... -- command [args...]
func cmdConfine(_ string, args []string) int {
	var writable []string
	i := 0
	for ; i < len(args); i++ {
		switch args[i] {
		case "--writable":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "hr confine: --writable needs a directory")
				return 2
			}
			writable = append(writable, args[i+1])
			i++
		case "--":
			i++
			goto run
		default:
			fmt.Fprintf(os.Stderr, "hr confine: unexpected argument %q\n", args[i])
			return 2
		}
	}
run:
	argv := args[i:]
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "hr confine: nothing to run after --")
		return 2
	}

	// Refusing beats running unconfined: the caller asked for confinement,
	// and silently not applying it is the control-believed-to-be-enforcing
	// failure this exists to remove.
	if err := sandbox.Apply(sandbox.Policy{Writable: writable}); err != nil {
		fmt.Fprintf(os.Stderr, "hr confine: could not confine, refusing to run: %v\n", err)
		return 2
	}

	path, err := lookPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hr confine: %v\n", err)
		return 2
	}
	// Exec rather than spawn: the confined process replaces this one, so
	// there is no unconfined parent left holding anything.
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "hr confine: exec %s: %v\n", argv[0], err)
		return 2
	}
	return 0
}

func lookPath(name string) (string, error) { return exec.LookPath(name) }
