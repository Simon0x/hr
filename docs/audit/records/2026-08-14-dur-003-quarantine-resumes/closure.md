# DUR-003 closure: a quarantined step can be resumed

**Closed:** 2026-08-14

## What the finding claimed

A failed step was re-derived and reclaimed forever, with no bound for
goal-less work. A watchdog quarantine mitigated the runaway cost, but the
finding recorded what defeated it: resolving the exception only wrote an
`intervention` ledger entry, so **there was no way to un-quarantine a step
key at all**. A human fixing the root cause got a permanently stuck step -
unbounded retry traded for unbounded block.

## What this pass found

Two of the three pieces had landed since the finding was written and the
register had not caught up. `pgstore.ResolveException` already resets a
quarantined job to `pending` when the exception carries a `stepKey`, the
predicate schema already has the field, and the watchdog's filer already
populates it.

The gaps were smaller and more specific:

1. **The watchdog could not quarantine at all in the situation it exists
   for.** `QuarantineRepeatedFailures` marked the newest *failed* row
   quarantined. But the repopulate loop reinserts a pending job after each
   failure, so by the time the threshold trips there is already a live row -
   and `quarantined` counts as active in `jobs_step_key_active_idx`. The
   update produced a second active row for one step key and failed with a
   unique-constraint violation. The watchdog errored out precisely when it
   was needed.
2. **Exceptions filed by `dispatch` carried no step key**, so the escalation
   and tool-blocked exceptions raised by `hr run` and the daemon could be
   resolved but not acted on.
3. **Three different digest recipes.** The job's key was
   `sha256(dept|because|input)`, `dispatch`'s exception subject was
   `sha256(dept+because+input)`, and the server's was
   `sha256(action+dept+input)`. No two agreed, so no exception subject could
   be matched back to the job it concerned.

## What this pass did

- `QuarantineRepeatedFailures` now quarantines the step's live row
  (`pending`/`claimed`/`running`) rather than a failed one. Only one active
  row per step key can exist, so it updates exactly one.
- `dispatch` puts `stepKey` in every exception predicate it files.
- One definition of a step's identity: both filers derive the subject digest
  from `dispatch.StepKey`, so three recipes that had to agree became one.

## Evidence

`internal/hrserver/quarantine_test.go` drives the whole loop against real
Postgres: a step fails three times, the watchdog quarantines it, the
exception it files carries the step key, resolving that exception returns the
job to `pending`. The test fails on the pre-existing code with the
unique-constraint violation described above, which is how the first defect
was found.

## What is left, and is not this finding

Goal-less work still skips the budget check - `dispatch.One` and `FanOut`
both no-op when `goal == ""`. The quarantine now bounds it after three
failures instead of never, which is what this finding asked for, but a step
with no goal above it still has no cost ceiling of its own. That is a budget
question rather than a retry one and is not carried here.
