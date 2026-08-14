# The Four-Layer Knowledge Model

Every durable fact about a project belongs to exactly one of four layers. A fact
in the wrong layer is worse than a missing fact: it is trusted, stale, and
load-bearing.

| Layer | Holds | Mutability | Read by | Drift signal |
|---|---|---|---|---|
| **C CONSTRAINTS** | Rules that must not break | Append + retire | Agents, every session | Its binding fails |
| **O ORIENTATION** | How the system currently is | Edited with the code | Humans + agents, on demand | Contradicted by code |
| **R REGISTER** | What is known-broken and open | Live worklist | Whoever is working | Entry closed but still listed |
| **H RECORD** | What happened | Frozen at write | Nobody, for truth | None - immutable by design |

The layers are not a filing convenience. They are four different *decay
profiles*, and mixing them is what makes a knowledge base rot.

**On the letters.** These layers were numbered `L1`-`L4` until the collision
became a cost: other control models in circulation - most immediately the
Development Framework's intent / generation / verification / authority stack -
also number their levels `L1`-`L4`, describing something entirely unrelated. A
reader holding both documents had no way to tell which taxonomy a bare `L2`
belonged to. Letters name the layer instead of ordering it, which is the honest
shape anyway: these are four decay profiles, not four rungs. `H` carries RECORD
because `R` was already REGISTER - read it as *history*.

Nothing in this framework numbers its levels now. The enforcement ladder names
its rungs (`compile`, `db`, `guard`, ...) for the same reason: a name survives
being quoted out of context, and a number does not.

---

## C - CONSTRAINTS

Rules whose violation the compiler will not catch. One line each. Lives in
`AGENTS.md` at the repo root; portable ones are inherited from the framework's
[`constraints/portable.md`](../constraints/portable.md) rather than copied, so a
project layer may only tighten them.

A constraint earns its place only if a reasonable engineer would otherwise make
the wrong edit. "Use TypeScript" is not a constraint. "Never cast a column to
text - the query generator loses NOT NULL" is.

See [`enforcement-ladder.md`](enforcement-ladder.md) - every constraint declares
how it is enforced, and the rung it sits on is the most useful fact about it.
Constraints that bind agent behaviour rather than code live here too.

---

## O - ORIENTATION

Current-state truth: architecture, conventions, development setup, API surface,
runbooks. The answer to "how does this work?", not "what must I not do?" and not
"what happened in June?".

Rules:

- **One file per subject, one subject per file.** A `CONVENTIONS.md` in `docs/`
  and another under a planning directory is two files holding one truth, and they
  will diverge.
- **Edited in the same commit as the code it describes.** Orientation that
  updates on its own schedule is H wearing a disguise.
- **No dates, no version numbers, no "as of".** A dated orientation doc is
  admitting it is a record. If it needs a date, it belongs in H.
- **Describes the present tense only.** Delete superseded text; git holds it.

Orientation is the audit's **expected-state oracle**. A delta analysis is
literally O compared against the code. Without a maintained O layer, every audit
re-derives the expected state from scratch before it can find a single delta -
which is how spec-delta analyses reach their twentieth revision.

---

## R - REGISTER

The single mutable list of open, known problems. One per project, at
`docs/audit/REGISTER.md`.

This layer replaces, and must absorb, every parallel worklist: concerns
documents, bug ledgers, the done-sections of a todo file, and the open items
scattered through closure reports.

### Finding schema

```markdown
### <ID>: <One-line claim>

- **Severity:** critical | high | medium | low
- **Issue:** what is wrong
- **Evidence:** `path/to/file.ts:120-140` - must resolve today
- **Risk:** what it costs if unfixed
- **State:** open | mitigated | closed
- **Fix:** the approach, not the diff
```

### The rules that keep it alive

- **Open items only.** A closed finding is deleted from the REGISTER the moment
  its closure lands in H. Never struck through, never annotated `DONE`.
- **Evidence must resolve.** Every path:line reference is checked at audit open.
  A finding whose evidence no longer exists is not "still open" - it is stale,
  and stale findings are how a concerns document comes to cite a dozen services a
  consolidation deleted months earlier.
- **Closing a finding is not enough.** It must first be asked whether the finding
  generalizes into a C constraint. That question is the entire point of the
  audit loop; see `audit-loop.md`.

---

## H - RECORD

What happened, frozen at the moment of writing: audit reports, delta analyses,
sprint closure reports, completion reports.

Lives at `docs/audit/records/YYYY-MM-DD-<slug>/`. Write-once. Never edited, never
consulted as truth about the present.

The discipline is entirely at the **boundary**: nothing durable may remain only
in H. Before a record is frozen, every fact in it is routed:

- a rule that must not break again → **C**, with a binding
- a description of how things now are → **O**
- a problem still open → **R**
- everything else → stays in the record, and is never read again

The characteristic failure looks like this: a batch report, marked done, with a
line buried in it saying the root test command is broken, here is the invocation
that actually works, and the testing doc is stale.

That is a live C constraint and an O defect sitting in a frozen record where no
agent will ever find it - while the testing doc, which every agent *does* read,
is knowingly wrong. One routing step at closure catches both.

It also explains why todo files drift into violating the pending-only rule: the
closure narrative has nowhere else to live, so it colonizes the worklist. Give H
a home and the R register stays clean.

---

## Layer assignment test

Three questions, in order. The first "yes" wins.

1. **Would a competent engineer break this without being told?** → **C**
2. **Is it broken right now?** → **R**
3. **Is it true in the present tense, and will it stay true until the code
   changes?** → **O**, otherwise → **H**

A fact that appears to belong in two layers is usually one constraint (C) plus
one explanation (O). Split it - the constraint must survive without the prose.
