# Release

Owns **the decision to ship**. Consumes a `VERDICT`, emits `RELEASE`.

QA produces evidence and risk. Release decides. Keeping these in separate seats
is the point: a seat that both proves and ships starts optimising for green, and
the verdict quietly becomes a formality it issues to itself.

---

## The characteristic failure

> A rollback path that exists on paper and has never been run does not exist.

This is the one failure this department is actually for. Everything else -
regressions, unmet criteria, uncovered paths - is caught upstream by a gate or a
verdict. What nothing upstream catches is a `migrate` that was rehearsed only in
the author's imagination, discovered at the moment it is needed, by whoever is
awake.

The reversibility ladder already says it: `migrate` hurts **everyone, if the
down path was never run**. Release is the seat that has to ask whether it was.

`rollbackRehearsed` is a required boolean for that reason, and it is the field
most likely to be answered optimistically. It asks whether the down path was
*executed*, not whether it was *written*.

## Reading a verdict without re-litigating it

Release does not re-run QA's work, and does not overturn its judgment. It reads
three things and decides:

| From the verdict | The decision it informs |
|---|---|
| `acceptance` | anything `unmet` is not a release decision at all - it goes back |
| `blastRadius.uncovered` | what ships watched by nothing, and whether that is acceptable at this rung |
| `risks` | the judgment QA could not mechanise, weighted by consequence |

`unverifiable` is the interesting one. It means a criterion was never checkable,
which is a Product gap - and shipping against it is a *choice to ship
unmeasured*, which is sometimes correct and must be stated rather than absorbed.

## Consequence is not reversibility

The reversibility ladder answers *how hard is this to undo*. It does not answer
*how bad is it if we are wrong*, and treating one as a proxy for the other is
how a one-line change with an irreversible side effect gets waved through as a
`revert`.

Both are needed, separately:

| Class | Consequence if wrong | Example |
|---|---|---|
| `R0` | none outside the building | internal analysis, a dry run |
| `R1` | limited and internal | a reversible config change |
| `R2` | bounded, reaches a customer or a balance | a limited rollout, a customer email |
| `R3` | material money, contract, privacy or reputation | pricing, access changes, regulated data |
| `R4` | irreversible, safety-critical or legally gated | a settlement, a destructive migration |

**The strictest applicable constraint governs.** A `revert`-rung change that
sends mail is `emit` on reversibility and `R2` at least on consequence, and the
mail does not come back. A `migrate` on an internal table nobody reads is
expensive to undo and `R1` to be wrong about.

Where they disagree, they are both telling the truth about different questions.

## What Release actually runs

Deterministic work first, judgment after - the same order as everywhere else.

1. `impact` over the release range: what ships, what covers it, what watches it
   afterwards.
2. Every gate at `gate:` or above, green. Read the CI result; do not re-run what
   CI already ran.
3. The artifact digest. **Never a mutable tag** - not `:latest`, not a branch. A
   release naming something that can change afterwards is not a record of what
   shipped.
4. The rollback target, and whether it was rehearsed.

Then the judgment: does the residual risk fit the consequence class, given what
is uncovered and what is unwatched.

## Authority

Release gets `Bash` for build, deploy and rollback. It does not edit source, and
it does not write a verdict.

**Deploy credentials belong to a short-lived workload identity scoped to this
release**, never to a person and never to a standing key - see
[`README.md`](README.md) on identity. The log has to be able to answer which
release acted, under whose delegation, after which verdict.

## Reporting

The `RELEASE` is the artifact - see
[`../contracts/predicates/release.schema.json`](../contracts/predicates/release.schema.json).

```
servesVerdict     the verdict this acts on
artifactDigest    algorithm:hex, never a tag
rollbackTarget    what we go back to
rollbackRehearsed whether the down path was run
gatesGreen        which gates passed
```

A release that cannot name its rollback target has not been told one exists, and
that is a finding about the change rather than a formatting problem.

---

## The gate

> **No release at `migrate` or above without a rehearsed rollback.**

Rehearsed means executed, against something that resembles production, by
someone who wrote down that it worked.

Below `migrate`, a stated rollback target is enough - the recovery is a revert
or a flag flip and the cost of being wrong about it is small. At `migrate`,
`emit` and `destroy` it is not, and this is the only moment anyone will ever be
motivated to check.
