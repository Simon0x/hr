package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(cmdDefault(root))
	}

	var code int
	switch args[0] {
	case "validate":
		code = cmdValidate(root, args[1:])
	case "test":
		code = cmdTest(root, args[1:])
	case "ledger":
		code = cmdLedger(root, args[1:])
	case "authority":
		code = cmdAuthority(root, args[1:])
	case "guard":
		code = cmdGuard(root, args[1:])
	case "confine":
		code = cmdConfine(root, args[1:])
	case "budget":
		code = cmdBudget(root, args[1:])
	case "impact":
		code = cmdImpact(root, args[1:])
	case "next":
		code = cmdNext(root, args[1:])
	case "run":
		code = cmdRun(root, args[1:])
	case "emit":
		code = cmdEmit(root, args[1:])
	case "sign":
		code = cmdSign(root, args[1:])
	case "keygen":
		code = cmdKeygen(root, args[1:])
	case "recall":
		code = cmdRecall(root, args[1:])
	case "remember":
		code = cmdRemember(root, args[1:])
	case "daemon":
		code = cmdDaemon(root, args[1:])
	case "exceptions":
		code = cmdExceptions(root, args[1:])
	case "worker":
		code = cmdWorker(root, args[1:])
	case "watchdog":
		code = cmdWatchdog(root, args[1:])
	case "identity":
		code = cmdIdentity(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		code = 2
	}
	os.Exit(code)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: hr <command> [subcommand] [flags]

  hr                  the only command a normal run needs: start everything
                       (postgres, server, worker, watchdog) if not already
                       running, attach if it is, open the browser. Ctrl+C stops
                       what this process started.
  validate contracts [--file <path>]
  validate attestation [--file <path>]
  validate ledger
  test contracts
  test attestation
  ledger append --kind <k> --actor <a> [--goal][--in][--out][--policy][--outcome][--detail][--tokens][--seconds][--model][--at]
  ledger verify
  ledger cost
  ledger show [--json]
  exceptions [--json]
  identity create --name <display name> [--departments <a,b,c>]
                       prints a bearer token, shown once - every /v1/ request
                       to hr-server needs one (HR_TOKEN, or pasted into the web UI)
  worker [--server <addr>] [--departments <a,b,c>] [--poll <duration>]
                       for a teammate's machine attaching to someone else's
                       already-running hr — not needed for your own
  watchdog [--threshold <n>] [--window <duration>] [--poll <duration>]
                       same - standalone use only, hr already runs one`)
}

// repoRoot walks up from the working directory looking for AGENTS.md — the
// first file any project adopting hr copies to its own root (see README.md
// "On a project", step 1), present whether this is hr's own source tree or
// an unrelated consuming repo that only has the compiled binary. go.mod is
// not a valid marker: it exists in hr's own module, never in a consumer's.
func repoRoot() (string, error) {
	// HR_ROOT wins when set. The guard runs as a hook inside the agent's own
	// working directory, which is wherever the step put it and need not sit
	// under the project at all, so walking upward from cwd finds the wrong
	// root or none. The dispatching process already knows the answer and
	// passes it down, following the HR_* convention every other
	// cross-cutting setting uses.
	if v := os.Getenv("HR_ROOT"); v != "" {
		if _, err := os.Stat(filepath.Join(v, "AGENTS.md")); err != nil {
			return "", fmt.Errorf("HR_ROOT=%s is not a project root: no AGENTS.md there", v)
		}
		return v, nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"could not find AGENTS.md upward from %s — hr expects it at the project root "+
					"(see README.md \"On a project\")", start)
		}
		dir = parent
	}
}
