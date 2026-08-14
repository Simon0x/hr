# Findings Register

**Open findings only.** A finding is deleted from this file the moment its
closure lands in `docs/audit/records/<date>-<slug>/`. Never struck through, never
annotated `DONE` - git and the record hold history.

Last OPEN-step validation: `<date>` · <n> entries · <n> stale evidence

---

## Critical

### <ID>: <one-line claim>

- **Severity:** critical
- **Issue:** <what is wrong>
- **Evidence:** `path/to/file.ts:120-140`
- **Risk:** <what it costs if left>
- **State:** open | mitigated
- **Fix:** <approach, not diff>

## High

## Medium

## Low

---

## Conventions

**IDs** are stable and never reused. Prefix by origin (`SEC-`, `DUR-`, `PERF-`)
so a finding stays traceable after it is promoted into a constraint.

**Severity** is consequence, not effort:

| | |
|---|---|
| critical | money is wrong, auth is bypassable, or data is silently lost |
| high | user-visible failure with no workaround, or a latent correctness bug |
| medium | degradation, missing bound, or real structural debt |
| low | worth remembering, not worth scheduling |

**Evidence must resolve.** Every `path:line` is checked when an audit opens. A
finding whose evidence has vanished is *stale* - possibly still real, but it must
be re-grounded before it counts. Unchecked evidence is how a register comes to
describe a system that no longer exists.

**Mitigated ≠ closed.** A mitigation that a misconfiguration can undo is still
open. Say what the mitigation is and what defeats it.

**Closing requires the promotion question:** will this class of bug recur, and
can a script detect it? That is step 5 of the audit loop - run `/hr:audit`, or
read `method/audit-loop.md` in the hr plugin.
