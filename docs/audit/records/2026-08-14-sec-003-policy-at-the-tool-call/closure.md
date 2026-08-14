# SEC-003 closure: the policy engine decides at the tool call

**Closed:** 2026-08-14

## What the finding claimed

`hr authority` returned allow/refuse/escalate but nothing required a caller
to consult it before acting — "an agent can simply not run it". The register
called this the review's own non-acceptance condition: policy expressed only
as instructions an agent may ignore, where "a control believed to be
enforcing is worse than one known to be advisory".

## What this pass found

Half of it had already been fixed and the register had not caught up:
`dispatch.One` evaluates policy before invoking, so a refused step never
reaches an agent. The remaining half was the more interesting one. That
decision fires **once**, for the whole invocation. The agent then makes as
many tool calls as it likes and the engine has no say in any of them — a step
scored `R1`/`revert` as a single unit could contain an action that is
neither.

The static tool grant was doing the constraining instead, and a flat
per-department allowlist is much weaker than a five-dimension engine.

## What this pass did

`internal/guard` runs the same engine against a single tool call:

- `ShapeOf` scores the call itself, not the step — read-only tools `R0`,
  file mutations `R1`, ordinary shell `R2`, and commands whose effect cannot
  be walked back (`rm -rf`, force push, `drop table`, `dd`) `R3`/irreversible.
  An unclassified tool scores as mutating: a tool nobody has classified is
  not evidence that it is harmless.
- `Decide` treats `escalate` as denial. There is nobody to escalate to inside
  a running invocation, and treating "a human should look at this" as
  permission is the failure the engine exists to stop.
- `hr guard` is the subcommand a harness runs per call. It fails **closed**:
  an unreadable policy, an unparseable payload, any error at all, denies.
- `internal/harness` writes a per-invocation settings file wiring Claude
  Code's `PreToolUse` hook to `hr guard`, in a temp dir removed with the run,
  and passes the step's situation through the child's environment. The hook
  is deliberately unmatched — deciding which tools are worth judging is the
  engine's job, not the wiring's.
- The situation travels on `context.Context` rather than in the `Harness`
  signature, because a fan-out runs several invocations at once in one
  process and process-wide state would have them overwrite each other.
- Refusals are written to a per-invocation file and folded into the ledger by
  the dispatching process afterwards. The guard does not append directly: the
  chain is serialized by an in-process mutex, and several guard processes
  appending to one chain would have nothing ordering them.

## Evidence

Verified end to end against the real CLI, not a mock. With the tool grant
explicitly allowing `Bash(rm *)`, an agent instructed to run
`rm -rf <path>/victim.txt`:

- the file survived,
- the refusal was recorded with the policy digest `84899498f1cf999d` and the
  engine's own reasons ("R3 caps autonomy at A1", "requested A2 exceeds the
  ceiling A1"),
- and the model was told it was a policy decision, not a tool error.

The grant would have permitted the call. The engine refused it. That gap is
the finding.

`internal/guard/guard_test.go` covers the scoring table, escalate-denies,
input summarisation, fold-exactly-once, and per-invocation tokens.

A bug the end-to-end run caught and that a mock never would have: the hook
process starts in the agent's working directory, which need not sit under the
project at all, so `repoRoot`'s upward walk found the wrong root or none.
`HR_ROOT` now wins when set, following the same convention as every other
`HR_*` setting.

## What is left, and is not this finding

Enforcement here is a policy decision, not confinement. A harness that did
not run the hook, or an agent reaching the filesystem by a route the harness
does not mediate, is SEC-004 — carried separately because it is a different
control at a different rung.
