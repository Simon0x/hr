# The Audit Loop

An audit is not an investigation. It is a **pass over the knowledge base**, using
C constraints as the checklist and O orientation as the expected-state oracle,
which ends by putting what it learned back into those same layers.

Run it that way and each audit is cheaper than the last, because every finding
either becomes a gate that runs in CI forever or a constraint that prevents the
next occurrence. Run it as a fresh investigation and you get
a twenty-ninth revision of the same delta analysis.

```
      ┌────────────── C CONSTRAINTS ◄───────────────┐
      │              (the checklist)                │
      ▼                                          PROMOTE
   1. OPEN → 2. SWEEP → 3. TRIAGE → 4. BATCH → 5. CLOSE
      ▲          │                      │           │
      │          ▼                      ▼           ▼
   O ORIENTATION            R REGISTER        H RECORD
   (expected state)       (open findings)    (frozen, once)
```

---

## 1. OPEN - validate the knowledge base before trusting it

Before looking for a single new problem, establish that the existing knowledge is
not fiction.

- **Resolve every `Evidence:` reference in the REGISTER.** Any `path:line` that
  no longer exists marks its finding *stale* - the finding may still be real, but
  its evidence is not, and it must be re-grounded before it is counted.
- **Run everything at `gate:` or above.** These are the constraints you already
  mechanized. Green means checks you do not perform by hand - that is the entire
  return on having built them.
- **List every `none` constraint.** These are your declared blind spots, and they
  are where an audit's manual attention is actually worth spending.

The output of OPEN is a scoped plan, not prose. If most of the REGISTER comes
back stale, stop: the audit's real finding is that the knowledge base decayed,
and re-grounding it is the work.

> A concerns document written before a service consolidation fails OPEN wholesale
> - it cites service paths that the consolidation deleted. Every audit that
> trusted it afterward was reasoning about a system that no longer existed.

## 2. SWEEP - find what the gates cannot

Only three things deserve manual sweep effort:

1. Constraints bound to `check:` - run the stated procedure.
2. Constraints marked `none` - the blind spots from OPEN.
3. **O versus code** - the delta analysis proper. Where does the architecture
   doc disagree with the architecture?

Anything at `gate:` or above is out of scope. If you find yourself hand-checking
something a script could answer, that is not a finding about the code - it is a
missing rung, and it goes in the REGISTER as one.

## 3. TRIAGE - one register, severity-ordered

Every finding lands in `docs/audit/REGISTER.md` in the R schema. Findings that
duplicate an open entry get merged into it, not appended beside it.

Severity is about consequence, not effort:

- **critical** - money is wrong, auth is bypassable, or data is silently lost
- **high** - a user-visible failure with no workaround, or a latent correctness bug
- **medium** - degradation, missing bound, or real structural debt
- **low** - everything else worth remembering

Nothing else is produced at this stage. A separate triage *document* is an H
artifact pretending to be a worklist; the REGISTER is the worklist.

## 4. BATCH - fix in commit-sized groups

Group by blast radius, not by severity, so each batch is one reviewable commit
with its own verification. A numbered block table - one row per block, with its
fix and its verification - is the shape that works.

Each batch states its verification up front. "Verified by" is a test that went
red→green, a gate that flipped, or a reproduction that stopped reproducing.
Not "manually checked".

## 5. CLOSE - the step that compounds

Closure is not "the fixes landed". It is the routing pass, and skipping it is why
audits repeat.

For **every** closed finding, answer two questions. The first is about
prevention, the second about detection, and skipping the second is why the same
class of incident keeps costing a full investigation.

> **1. Will this class of bug recur, and what is the highest rung that can hold
> it?**

| Answer | Action |
|---|---|
| The unsafe call can be made untypeable | Restructure. Add the constraint, `compile`. |
| The database can refuse it | Trigger, role, or constraint. Add it, `db`. |
| A script can detect it over a derived target set | Write the guard. `guard:` |
| A script can detect it over a fixed target | Write the gate. `gate:` |
| Needs human judgment | Add the constraint, `check:`, with the procedure. |
| No viable check today | Add the constraint, `none`. Honest beats silent. |
| Genuinely one-off | Nothing. Let it die in the record. |

> **2. If it recurs anyway, will anyone find out sooner?**

Most incidents cannot be prevented, and nearly all of them can be detected one
rung faster on the observability ladder in `../departments/ops.md`. A finding
closed at "we fixed it" bought a fix. One that moves a failure mode from `logs`
to `alerts`, or from `alerts` to `refuses`, bought every future occurrence.

Then, in order:

1. Promote durable rules into **C** with bindings.
2. Correct **O** wherever the audit proved it wrong.
3. **Delete** closed findings from the **R** register.
4. Freeze the narrative into **H** at `docs/audit/records/YYYY-MM-DD-<slug>/`.
5. Report **coverage** - constraints at `gate:` or above ÷ total. If nothing
   climbed a rung, the audit bought you nothing durable.

### What promotion looks like

A forensic audit into accounting drift produces, instead of a report, a named
invariant battery: `notional ≈ |volume × price|`, unrealized P&L consistent with
open and current price, each check exiting non-zero on violation. It then runs
forever, against every seeded database, for free.

That is the whole return on the loop, and it is the single most valuable artifact
an audit can produce. It tends to happen spontaneously once and never again -
because nothing in the process asks for it a second time. Step 5 asks every time.

---

## Cadence

Full audits are expensive and their value decays if the gates are working. Run
one when a **structural** change lands - a consolidation, a persistence swap, an
auth model change - or when constraint coverage stalls.

Do not run a full audit to find regressions. That is what the gates are for, and
if they are not catching regressions, fix the gates.
