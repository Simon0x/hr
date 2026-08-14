# DUR-001 closure: every constraint names how it is enforced

**Closed:** 2026-08-14

## What the finding claimed

Five `AGENTS.md` constraints bound at `check:` with no script behind them, so
`impact` could not derive which paths they govern and reported them as
repo-wide. A constraint that applies everywhere discriminates nowhere.

## What this pass did

Four of the five already had enforcement and only lacked a binding that said
so. Naming what is actually there is not the same work as building it, and
conflating them is how a coverage number stops meaning anything:

- **"A goal states a budget, and the check happens before work starts"** and
  **"the dispatcher asks the cheap question first"** → `test:`, backed by
  `TestOne_BudgetRefusalHappensBeforePolicyAndTheModel`, which asserts an
  unaffordable step leaves `StepResult.Decision` nil: policy was never
  evaluated, and the model was never reached.
- **"Acting is opt-in"** → `test:`, backed by
  `TestOne_DryRunReachesADecisionAndInvokesNothing`, which also asserts a dry
  run still records its decision. A plan that leaves no trace is not a plan.
- **"Every event names the identity that caused it"** → `gate:
  validate:ledger`, which already enforced it: the event schema requires a
  non-empty `actor` on every entry, and the gate validates every line against
  it.

The fifth needed work first. **"Cost is reported per accepted outcome, never
as raw tokens"** was true of the output but had no seam to assert against —
the per-goal arithmetic ran inline in `cmd/hr/budget.go`. It moved to
`budget.Report`, which returns spend, outcome and accepted counts, human
touches, and a `PerAccepted` guarded by `Denominated`.

That last flag is the constraint in code: with nothing accepted there is no
rate, and the report says so rather than printing a spend figure that reads
like one. `TestReport_RefusesToDivideByNothingAccepted` is the test that would
fail if someone made zero-accepted spend look like a cost.

## Evidence

`hr validate constraint-bindings` derives coverage rather than trusting the
stated line, and reports **26/30 at gate or above (87%)**, up from 70% at the
start of the day. No constraint carries `want:` any more — the backlog line
the gate prints is empty for the first time.

The four remaining `check:` bindings are genuine: they describe review habits
(read the scaffold end to end, grep for consuming-project identifiers) that no
program checks, and they say so rather than claiming a rung they do not hold.
