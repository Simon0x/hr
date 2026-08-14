# DUR-002 closure: goal-naming-an-open-finding routes to Engineering

**Closed:** 2026-08-13

## What the finding claimed

`internal/workflow.DeriveFrom` routed every `GOAL` through Discovery unless a
`SPEC` already existed above it, even when the goal named an open register
finding directly - `departments/README.md`'s documented "second queue" into
Engineering. A 2026-08-12 note on the finding observed that the GOAL-stage
code already appeared to route open-finding goals straight to Engineering,
but nobody had confirmed it with a test or landed the closure, so the
register kept carrying it as open.

## What this pass did

Added `internal/workflow/workflow_test.go` (`internal/workflow` had zero
test files before this), including two tests that exercise exactly the
claim above:

- `TestDeriveFrom_GoalNamingOpenFinding_RoutesStraightToEngineering` - a
  goal whose `serves.findingId` is open in the register produces an
  Engineering-ready step reading "closes open finding ... — the second
  queue, no spec needed", and asserts no Discovery step is produced for it.
- `TestDeriveFrom_GoalNamingClosedFinding_IsBlockedNotRouted` - a goal
  naming a finding that is *not* open produces a Blocked entry instead.

Both pass against the code as it stands. The 2026-08-12 note was correct:
the fix was already in `internal/workflow/workflow.go`, it just had no
test proving it and no closure landed. This record and the register
deletion are that closure, not a code change.

## Promotion (audit-loop step 5, question 1: will this recur, and what's
## the highest rung that can hold it?)

Regression protection now sits at `test:` - a named test that fails on
regression:
`test: internal/workflow.TestDeriveFrom_GoalNamingOpenFinding_RoutesStraightToEngineering`,
wired into CI unconditionally via `.github/workflows/validate.yml`'s "Go
unit tests" step (`go test ./...`, no `continue-on-error`). Anyone deleting
that test is the only way to lose the protection.

No new `AGENTS.md` constraint was promoted. This finding was a specific
routing-logic correctness bug, not a rule a competent engineer would
otherwise violate without being told (the layer-assignment test in
`method/knowledge-layers.md` routes it to R, closed, not C) - the test
suite this pass added is the durable artifact, not a new constraint bullet.
Climbing past `test:` (to `guard`/`db`/`compile`) would need the routing
logic itself restructured into something a static check or the type system
could verify, which is disproportionate to a single confirmed-correct
branch.

## Orientation check (step 5, question 2 in spirit; step "correct O")

`departments/engineering.md`'s two-queues section, cited as the finding's
evidence, was already accurate - it describes the code's actual behavior
now confirmed by test. Nothing in O needed correcting.

## Detection (step 5, question 2: if it recurs anyway, will anyone find out
## sooner?)

Before this pass: nobody, until a human noticed goals with open findings
sitting in Discovery's queue instead of Engineering's - a `silent` failure
mode. After: `refuses` - a CI build failure on the next PR that regresses
the routing, since `go test ./...` is unconditional in `validate.yml`.
