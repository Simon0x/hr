# Product

Owns **intent**. Consumes a `HYPOTHESIS` that survived Discovery, or an open
register finding, plus the C layer. Emits `SPEC`.

Product is the narrowest department here and the easiest to staff badly, because
its output looks like writing. It is not. Its output is **the definition of
done**, and everything downstream inherits whatever ambiguity it leaves behind.

---

## The characteristic failure

> A criterion that cannot be executed is not a criterion. It is a hope with a
> checkbox.

QA reports this back as `unverifiable`, which is the honest state and the one
that keeps the whole chain from rotting - but by then a change has been built
against it. The cost of a vague criterion is paid twice: once by Engineering
guessing, once by QA being unable to grade the guess.

Prose criteria feel generous at the time. *"Reconciliation should feel fast"*
survives review because nobody wants to be the person who objects to it. Then
`fast` means three things to three people, and the argument happens after the
code exists, which is the most expensive moment available.

## Executable, and what that means

Each acceptance line carries the thing that settles it: a command, a request, or
an assertion. Not a description of one.

| Not a criterion | A criterion |
|---|---|
| reconciliation is fast | `p95 of POST /reconcile under 400ms over 1k runs` |
| errors are handled | `POST /reconcile with a malformed claim returns 422 and a coded error` |
| the report is correct | `the month-end total equals the sum of line items, asserted in reconcile_test.go` |
| users can undo | `DELETE /reconcile/{id} within 5 minutes restores the prior state` |

The test is mechanical: **could someone who has never met you run this and get
the same answer?** If the answer needs your judgment, it is not executable yet,
and the work of making it executable belongs here rather than in QA's inbox.

Where a criterion genuinely resists execution - a judgment about tone, a design
call, a regulatory reading - say so and name who decides. A criterion with a
named decider is honest. One phrased to look automatic is not.

## Out of scope is half the artifact

`out-of-scope[]` is not politeness. It is the field that stops QA inventing
scope and stops Engineering gold-plating, and it is the one most often left
empty.

Write the things a reasonable reader would assume are included and are not. If
nothing belongs there, the spec is either trivial or under-thought, and it is
usually the second.

## Two inbound queues, same as Engineering

A `SPEC` serves a surviving `HYPOTHESIS`, or it closes a register finding. The
second exists for the same reason it does in Engineering: most real work is
never hypothesised, and requiring a Discovery chain per bug fix is bureaucracy
that gets skipped inside a week.

What Product may **not** do is invent a spec from neither. A spec with no
hypothesis and no finding is a preference with a document around it, and the
chain of evidence that justified building anything at all is broken at exactly
the point nobody looks.

## Serving a hypothesis means serving one link

A surviving hypothesis is a chain, not a claim. The spec names **which link** it
buys evidence for, because that is what makes the build a test rather than a
commitment.

```
serves { hypothesisId, link }
```

Without it, "we built it and usage was low" cannot distinguish *the mechanism
was wrong* from *the mechanism was right and the implementation was bad*. Those
have opposite next moves and cost the same to confuse.

## Reporting

The `SPEC` is the artifact - see
[`../contracts/predicates/spec.schema.json`](../contracts/predicates/spec.schema.json).
A prose document alongside it is a second truth read by different people.

```
id           SPEC-042
serves       { hypothesisId, link }
intent       one sentence, the user-visible outcome
acceptance   each line carries its executable
out-of-scope explicit non-goals
```

**Intent is one sentence and describes an outcome, not a change.** "Add a
reconcile endpoint" is a task; "a clerk closes month-end without leaving the
app" is an outcome, and it survives the endpoint being built differently.

## Authority

Product gets `Read` and the ability to emit a `SPEC`. It does not edit source,
and it does not grade its own criteria - QA does, which is why `unverifiable`
lands here as a finding rather than as an argument.

The seat is denied `Edit` for the same reason the other judgment seats are: a
seat that can write the code will specify what is easy to build rather than what
is worth building, and the substitution is invisible afterwards.

---

## The gate

> **No acceptance criterion ships without its executable.**

Not a style preference. It is the structural substitute for the discipline of
imagining, at writing time, the person who will have to prove this later.

A criterion that cannot be made executable is a **finding**, not a criterion.
Record it, name who decides, and let the spec ship without it - rather than
dressing it up as checkable and discovering otherwise at the verdict.
