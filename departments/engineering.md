# Engineering

Owns **the change**. Consumes a `SPEC` or an open register finding, plus the C
and O layers. Emits `CHANGE`.

The diff is not the scarce thing. Generation is cheap and getting cheaper, while
review attention and recovery cost are not - so a department organized around
producing diffs is optimizing the one resource that stopped being constrained.
What Engineering owes is a change that is **conformant, reviewable and cheap to
undo**, and that leaves the knowledge base truer than it found it.

It is also the only department holding `Edit`, which makes most of this document
a statement of what a write may not do and what it must carry with it.

---

## Two queues, and only two

> A change serves a `SPEC`, or it closes a finding in the register.

Nothing else is a legal inbound. Most real work - a dependency bump, a refactor,
a fix - is never spec'd, and the alternative of asking Product for a micro-`SPEC`
per bump is bureaucracy that gets skipped inside a week. The R register is
already the other worklist; making it a first-class queue costs nothing and
keeps unbudgeted work visible instead of absorbed.

Work arriving through neither door is not forbidden. It is **unattributed**, and
it gets said out loud in the brief, because a department that quietly absorbs it
cannot tell you where its capacity went.

This is also why Engineering can start before Product exists. The register queue
needs no upstream department.

---

## The intake

Intake is where an agent earns its place: judging intent, weighing risk, reading
a spec. The derivations around it are scripts. The two interleave, and the
interleaving is the procedure.

### Work a frontier, not a questionnaire

A fixed list of questions rots the way any hand-maintained list rots, and it asks
everything at uniform depth regardless of stakes. Same structure as Discovery,
over a different object: there, leads about a market; here, unknowns about a
change.

Cheapest first. Each answer settles something, spawns a better question, or both.
Depth scales with the rung - a flag flip does not earn question six.

### The questions, cheapest first

| # | Question | Why here |
|---|---|---|
| 1 | Which queue - a `SPEC`, or a finding? | Costs one question and kills the most work |
| 2 | Is this already an open finding? | It merges into that entry, never lands beside it |
| 3 | What rung is this? | The rung selects everything downstream |
| 4 | What constraints bind it? | **Derived, not asked.** The agent stops asking and starts telling |
| 5 | What is the revertible version? | See below |
| 6 | What breaks if this is wrong, and how would anyone find out? | The failure-mode table |
| 7 | What does O say afterwards? | The design delta |

> **What is the revertible version?**

That is the department's characteristic question, and the one that pays. Changes
get proposed at `migrate` when a `flag` version exists that answers most of the
question in a day. Nobody asks it unprompted, on their own work, ever.

### Two modes

**A change is proposed.** Interrogate it downward: smaller, lower rung, later
commitment.

**A problem is named and no change is proposed.** Produce candidates at different
rungs and state the trade: one at `flag` that costs a day and settles most of it,
one at `expand` that costs a week, one at `migrate` that cannot be undone. This
is the mode that decides what to do rather than checking what was decided, and it
is the reason intake sits upstream of the work rather than in front of the merge.

### Derived, declared, and the difference

Asking someone whether their own change is reversible returns yes, for the same
reason "would you use this?" returns yes. It is the `stated` rung wearing
engineering clothes.

**The rung is derived from the diff wherever it can be, and declared only where
it cannot.** Migrations present, destructive SQL, a new outbound client, whether
a flag was registered - all readable. Whether the down path was ever actually run
is not, so it is declared, and the declaration is the first thing review looks
at.

A declared rung that the derivation contradicts is a **finding**, not an
override.

### Blast radius is a range

Depth is a choice, so a single answer is false precision. Report bands:

| Band | Confidence |
|---|---|
| direct paths in the diff | certain |
| transitive reach through the import graph | likely |
| runtime reach - dynamic dispatch, config-driven wiring, queue consumers | unknown, and named |

The third band is where production incidents come from and no tool closes it.
Naming it is the honest output; a clean two-band answer reads as *checked*.

Each band carries two coverage columns, not one: **a gate that watches it before
deploy, and a signal that watches it after.** A path with gates and no signals
ships confidently and fails silently. See [`ops.md`](ops.md).

### The brief

Write a document, not a transcript. **Never report the questions** - they are
working notes, exactly as the chain is in Discovery.

```markdown
# <the change, restated in one line>

**<Which version to build, at which rung, roughly what it costs. Two sentences,
standing alone. Someone who reads only this should be able to act.>**

## What this touches

| Path | Binds | Gate | Signal |
|---|---|---|---|

## What it costs to be wrong

Rung, rollback path, and what an undo actually requires of whoever performs it.

## Failure modes

| Mode | Signal | Rung |
|---|---|---|

## Not decided

| Question | Would cost | Why it matters |
|---|---|---|
```

Sentence-case headings. One fact per row. **Not decided** is the analogue of
Discovery's *not checked*: an unanswered question is a finding, and silence about
one reads as answered.

### Intake does not build

The intake seat has no `Edit`, and that is load-bearing rather than tidy. An
agent that can intake and then immediately code will rush the intake to reach the
coding - the same mechanism by which a QA agent that can fix quietly fixes and
reports success. The brief hands off to a working session; it does not become
one.

---

## The reversibility ladder

Every department here holds its claims to a ladder, and the rung always names who
could still be wrong. For a change the question is **who is hurt if this is
wrong**, which is reversibility.

| Rung | Means | Who is hurt if it is wrong |
|---|---|---|
| `revert` | one commit reverts it cleanly | nobody |
| `flag` | dark by default, switchable per tenant | whoever it is on for, until it is flipped |
| `expand` | additive only; old and new paths both live | nobody yet - the contract step is a separate change |
| `migrate` | schema or data moved, down path tested | everyone, if the down path was never run |
| `emit` | an external side effect left the building - mail, payment, webhook | the recipient, permanently |
| `destroy` | data gone, no down path | everyone, permanently |

Engineering's characteristic failure is not wrong code. Gates catch regressions
and QA catches unmet acceptance. It is **unrecoverable** wrong code, which is why
this is the axis worth ranking on.

### The gate

> **No change combines two rungs.**

A change's recovery cost is set by its highest rung, so bundling a feature with
its migration means the cheap half cannot be reverted without the expensive one.
Expand and contract are two changes. A migration does not ride along.

This is the one rule here that changes what actually gets built, and it is the
structural substitute for an instinct that only arrives after an incident.

### What the rung selects

One classifier, and the process falls out of it - rather than a uniform process
that is simultaneously too heavy for a flag flip and too light for a migration.

| Rung | Design artifact | Review |
|---|---|---|
| `revert` `flag` | none - folded into the change | the diff |
| `expand` | the O diff | the diff, plus what O will say |
| `migrate` `emit` `destroy` | `DESIGN`, **before** code | the design, then the diff |

---

## Design is a diff against O

A design document and an orientation document are the same document at two points
in time. Written as a separate artifact it is frozen at proposal, still read as
truth a year later, and is H wearing a disguise - the exact failure the
knowledge model exists to prevent.

> Engineering does not write a design document. It writes a **diff against O**
> in the future tense, and the change that lands the code is the change that
> flips it to present tense.

There is then no second artifact to forget to update, and design review asks the
right question: *what will this repo believe about itself afterwards?*

The parts that are not O-shaped route exactly as a closed finding routes:

| The design says | Goes to |
|---|---|
| this is how the subsystem works | **O**, as the diff |
| this must never be broken later | **C**, with a binding |
| this is known-wrong and shipping anyway | **R** |
| we chose X over Y, and here is the trade | **H**, never read as truth again |

**The C layer is the standing technical spec.** A design is only ever the delta
against it, which is why question 4 is derived and delivered at the point of
work. A
constraint someone has to remember is at `check:` rung at best, and memory is the
weakest binding on the ladder.

### DESIGN

Never crosses a department boundary - Engineering writes it and Engineering
consumes it - which is why it is not a row in the spine.

```
DESIGN  v, serves{spec-id | finding-id}, rung, o-diff,
        constraints{binds[], proposes[]},
        failure-modes[{mode, signal, rung}],
        rollback
```

`constraints.proposes[]` is how a change argues that an existing constraint is
wrong. It is a proposal, and it is resolved before the code lands - see
Authority.

---

## Every failure mode gets a signal

`failure-modes[]` is a required field, and requiring it is the entire mechanism
by which instrumentation stops being an afterthought.

> A change that adds a failure mode without adding the signal that detects it is
> incomplete.

Symmetric to QA's *uncovered is a finding*: QA asks which paths nothing watches
in CI, Ops asks which failure modes nothing watches in production. Same question,
different clock.

Each mode carries a rung from the observability ladder in [`ops.md`](ops.md).
`silent` is permitted and must be **declared**, exactly as `none` is on the
enforcement ladder. A mode listed as `silent` is a blind spot on the record; a
mode not listed at all is a blind spot nobody knows about.

---

## Authority

Engineering gets `Read`, `Edit`, `Bash`. It lacks deploy. Two further
restrictions, both of which exist because an agent will otherwise take the
shortcut:

- **No write to C in a change that touches source.** An agent blocked by a
  constraint edits the constraint. It is the most predictable failure of handing
  a model `Edit`, and it is invisible in review because the diff looks
  self-consistent. Engineering may *propose* a constraint change via
  `DESIGN.constraints.proposes[]`; it may not land one alongside the code that
  needed it. `check:` today, `want: guard` - a diff touching both C and source
  fails unless explicitly flagged.
- **No write to H.** Records are frozen at write and Engineering does not author
  history.

### The pattern in the authority table

Three seats now lack the tool that would let them shortcut their own judgment:
Discovery's kill seat has no `Write`, QA has no `Edit`, intake has no `Edit`.
That is not three coincidences. **A seat whose job is to think clearly about work
is denied the ability to do that work**, because the ability is always the
cheaper path and it always wins under time pressure. Apply it when adding the
fourth.

---

## Closing a change

Closure is not "it merged". It is the routing pass, and it is the same pass the
audit loop runs at step 5 - run here, per change, while the knowledge is still
recoverable.

**Surprise is a finding.** Every point where O said something untrue, a
constraint was ambiguous, or the work turned out to be re-derivation a guard
should have done, costs nothing to record at the moment it happens and is
unrecoverable an hour later. Discarding it is how audits come to re-derive at
full price, weeks late, what was free at the time.

For each one, in order:

1. A rule that must not break again → **C**, with a binding.
2. A description of how things now are → **O**, in this commit, not the next.
3. A problem still open → **R**.
4. Anything else → it dies here, deliberately.

Then emit the `CHANGE` with `routed{}` populated. An empty `routed{}` is a
legitimate answer on a small change and a suspicious one on a large change.

The audit loop remains the fallback for what this pass misses. It should be
finding less over time, and if it is not, this step is not being run.
