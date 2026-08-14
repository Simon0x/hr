# The Enforcement Ladder

Every constraint declares how it is enforced. A constraint without a binding is
folklore.

```markdown
- Never cast a column to text in query SQL - the generator loses NOT NULL.
  Parameter casts are fine. `guard: validate:sql-casts`
```

Enforcement is a ladder, not a binary. Each rung is strictly weaker than the one
above, and the rung a constraint sits on is the single most useful fact about it -
because it says who can still violate it.

| Binding | Means | Who can violate it |
|---|---|---|
| `compile` | The wrong code does not build | Nobody |
| `db` | The database refuses the operation | Nobody, including other clients |
| `guard: <script>` | Static analysis over a **derived** target set | Anyone who skips CI |
| `codegen` | Generated and diffed; drift is a type error | Anyone who edits generated output |
| `test: <name>` | A test fails | Anyone who deletes the test |
| `gate: <script>` | A CI script exits non-zero | Anyone who skips CI |
| `check: <procedure>` | A human follows a stated procedure | Anyone in a hurry |
| `none` | Declared blind spot | Everyone |

**`want: <rung>`** is orthogonal: this could sit higher and nobody has built it
yet. That is the standing backlog, and where audit effort converts into permanent
leverage.

## Climb it, don't just occupy it

Every audit asks of each constraint: *can this go up one rung?* A `check:` that
could be a `gate:` is wasted human attention. A `gate:` that could be a `compile`
is a bug waiting for a deadline.

**Coverage** - the share of constraints at `gate:` or above - is the one health
metric. It rises over time or the audits are not paying for themselves.

## Why the top rungs are worth it

> Make the dangerous path a compile error, not a code review note.

Take an audit-context requirement: every mutating write must run inside an
audited transaction.

- As `check:`, it is a review note that gets missed.
- As `compile`, the write pool is wrapped in a type satisfying neither the query
  interface nor the transaction API. The unaudited call does not build. Exactly
  two doors exist: the audited path, and a deliberately greppable escape hatch.
  An entire bug class becomes a build failure.
- Below it, `db`: a trigger that raises when the audit context is missing, and a
  SELECT-only role for read binaries. The app cannot forget, because the database
  refuses.
- Below that, `guard`: a static check deriving its target set from schema
  declarations plus query files at scan time, run from one source in three places
  - the lint task, the pre-push hook, and CI.

> Self-updating target sets matter; a hardcoded list rots in a month.

That is the whole reason `guard` outranks `gate`. A gate with a hand-maintained
list of files to check is a hand-maintained document, and it decays like one.

## The rung is not decoration

The clearest case: a constraint written down correctly, then audited by hand
three times, each pass re-deriving the same answer and producing a document
nobody reads - when the check was a regex all along.

Nothing in the process ever asked *"can this go up a rung?"* The binding asks it,
on every constraint, every audit.

## Where the ladder does not apply

Constraints that bind **agent behavior** rather than code:

```markdown
- **NEVER run `task db:migrate` or `task dev`.** Wait for the user.
  (`task db:diff` is allowed - it only writes local files.)
```

These sit at `check:` by nature, enforced by the agent reading them. That makes
placement load-bearing: near the top of the C layer, phrased as prohibitions.

What makes them hold is stating what *is* allowed alongside what is not. A rule
banning a whole command family gets ignored the first time someone legitimately
needs a migration generated.

## Beyond code

The ladder applies to organizational boundaries too. A tool grant that withholds
`Edit` from a QA agent is `compile`-rung enforcement of separation of duties: the
wrong action is impossible, not merely forbidden. See `../departments/README.md`.
