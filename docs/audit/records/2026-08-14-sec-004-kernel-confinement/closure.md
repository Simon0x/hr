# SEC-004 closure: confinement that does not need the harness to cooperate

**Closed:** 2026-08-14

## What the finding claimed

hr decided a seat's authority and passed it correctly, but every bit of
enforcement was the harness's. A grant of `Bash(hr emit *)` held only because
Claude Code's matcher honoured it; the per-call policy guard held only
because the harness ran the hook. Neither helps against a harness that is
broken, or one that wrongly declares it can enforce. There was no sandbox, no
filesystem fence, no process restriction of hr's own.

## What this pass did

`internal/sandbox` is the seam: a `Policy` naming the roots a process may
write under, a `Sandbox` that wraps argv before spawn, and two providers -
`Landlock` on Linux and `None` elsewhere, which reports its own name so an
unconfined deployment is a stated fact rather than a missing control.

Landlock was chosen because it needs no privileges and no setuid helper,
which is what makes kernel-enforced confinement reachable from a tool that
runs as an ordinary user. It is the same mechanism DeepSeek Harness ships
`native/landlock-run` for.

hr is its own helper. A Landlock ruleset applies to the calling process and
Go offers no hook between fork and exec, so `Landlock.Wrap` re-enters hr as
`hr confine --writable DIR... -- <argv>`; that process restricts itself and
then execs the real command. The agent therefore starts already confined and
cannot un-confine, because a ruleset is irrevocable. `hr confine` refuses to
run at all if it cannot apply the ruleset - running unconfined after being
asked to confine is the failure this closes.

Reads and execution are deliberately unrestricted. The denials a seat
actually declares are `Edit`, `Write` and `NotebookEdit`, so write
confinement is the control that matches them, and restricting reads breaks a
toolchain long before it stops anything.

Writable roots are the project, the scratch directories every toolchain
assumes, the device nodes a shell needs, and the agent product's own state.
`HR_SANDBOX=off` disables it, because a confinement that cannot be turned off
gets worked around in ways nobody records.

## Evidence

Verified against a real agent, not a mock. `claude` was spawned through
`hr confine` and asked to write two files:

- `probe.txt` in its workspace — written, containing `OK`.
- `/etc/hr-escape.txt` — refused by the kernel, `EACCES`, reported by the
  agent itself as "blocked at the OS permission level".

The agent otherwise ran normally: confinement did not break the toolchain.

`internal/sandbox/sandbox_test.go` re-executes the test binary as a confined
child and asserts a write inside the writable root lands while a write
outside does not. Landlock enforcement was probed on this kernel before any
of it was built (ABI 7), rather than assumed.

## What is left, and is not this finding

Confinement here is filesystem-only. Network egress is unrestricted, so an
agent can still reach the world even when it cannot write to it - which is
the path SEC-003's untrusted-content work fences rather than blocks. A
provider for egress belongs behind the same seam and is not carried as an
open finding until there is a reason to build it.

On a platform without Landlock the seam selects `None` and says so; nothing
claims confinement it does not have.
