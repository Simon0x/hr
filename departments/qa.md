# QA

Owns **proof**. Consumes `CHANGE` + `SPEC`, emits `VERDICT`.

Two questions, and they are not the same question:

1. **Acceptance** - does this do what the spec said?
2. **Blast radius** - did it break anything else?

Acceptance is judgment; it needs a spec and a reader. Blast radius is derivable;
it needs a diff and a coverage map. Build the second first - it is mechanical and
it pays on every change.

---

## Blast radius

> Which gates cover the paths this diff touched, and which touched paths no gate
> covers at all?

The second half is the valuable one. A green run over paths nothing watches is
not evidence, and that is the state most changes ship in.

### Coverage is declared, not curated

Each gate script declares what it covers, in its own header:

```ts
/** @covers services/*/src/**/*.queries.sql */
```

`impact` scans gate scripts for `@covers`, builds the map at run time, and
intersects it with the diff. Nobody maintains a list.

A hand-written path→gate map is a hand-maintained document and rots like one.
This is the same reason `guard` outranks `gate` on the ladder.

### The other half of the radius

A gate watches a path **before** deploy. An alert watches it **after**, and the
two coverage sets are not the same one. A path with gates and no signals ships
confidently and fails silently, which is the more expensive of the two failures.

QA owns the pre-deploy half and reports it. The post-deploy half is declared the
same way, derived by the same tool, and owned by [`ops.md`](ops.md) - so when a
verdict lists an `uncovered` path, check whether it is also unwatched before
deciding how much the gap costs.

### Output

```
covered      path → gates that ran → pass/fail
uncovered    path → nothing watches this      ← the finding
gate-only    gate ran but its paths were untouched (noise; drop it)
```

`uncovered` paths are the standing backlog. A change touching an uncovered path
is not blocked - it is *flagged*, and the flag is what eventually becomes a gate.

### Depth

Direct path matching finds the obvious. Real blast radius follows the import
graph: a change to a shared package reaches every consumer. Start with direct
paths, add transitive reach when the first shared-package regression escapes -
that incident is what tells you the depth you actually need.

---

## Acceptance

Blocked upstream when specs are prose. "Fulfils the spec" cannot be answered
against a document that never said what "fulfilled" looks like.

The minimum viable `SPEC` for QA to work against:

```
id           SPEC-042
intent       one sentence, the user-visible outcome
acceptance   - [ ] each line executable: a command, a request, an assertion
out-of-scope explicit non-goals, so QA does not invent scope
```

If a criterion cannot be written as something checkable, that is a Product
finding, not a QA failure. Say so in the verdict rather than guessing.

### Verdict on each criterion

- **met** - with evidence: the test that went red→green, the request and response
- **unmet** - with what was observed instead
- **unverifiable** - the criterion is not checkable as written. Names a Product gap.

`unverifiable` is the honest state that keeps the whole thing from rotting. Never
grade an unwritten criterion as met.

---

## The verdict is not a merge decision

QA emits evidence and risk. Release decides. Keeping these separate matters
because a QA agent that also gates the merge starts optimizing for green.

```
VERDICT
  spec-id
  acceptance    [{criterion, met|unmet|unverifiable, evidence}]
  blast-radius  {covered[], uncovered[], failed[]}
  risks         [{what, why, confidence}]
```

`risks` is where judgment goes - the things no gate covers and no criterion
named. Ordering effects, migration reversibility, a shared type that changed
meaning without changing shape. State confidence honestly; a hedged real risk is
worth more than a confident fabricated one.

---

## Agent constraints

- **No `Edit`.** QA reports; Engineering fixes. An agent that can fix what it
  finds will fix it and report success, and the finding disappears.
- **A finding without evidence is not a finding.** Path, line, command, or output.
- **Do not re-run what CI already ran.** Read the CI result. Re-running gates
  locally is the cost the gates were built to remove.
- **Uncovered is a finding.** Silence about unwatched paths reads as "checked".
