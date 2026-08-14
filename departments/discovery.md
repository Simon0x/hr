# Discovery

Owns **whether to build**. Consumes `SIGNAL`, emits `PROBLEM` and a resolved
`HYPOTHESIS`. Product turns the survivors into `SPEC`.

An unfilled Discovery seat does not stop work - it makes work start without
evidence, silently. The gate at the bottom of this file is what makes the empty
seat visible.

---

## The missing artifact

Most stalled builds skip from a **segment** to a **solution** with no problem
stated in between:

> *"independent pharmacies"* → *"a shared claims clearing hub solves it"*

There is no *it*. Nobody wrote down who hurts, how badly, what they do about it
now, and what that costs them - so "does this solve it" is unanswerable, and the
question quietly becomes "is this buildable", which is always yes.

```
SIGNAL → PROBLEM → HYPOTHESIS → EVIDENCE → SPEC
```

### PROBLEM

```
who        the specific role that hurts, not the company
when       the situation that triggers it
cost       what it costs them now, in money, time or risk
workaround what they already do about it
```

**The workaround is the proof.** People in pain improvise - a spreadsheet, a
part-time hire, a WhatsApp group, a rule of thumb. No improvised fix means no
pain worth paying to remove. A problem with no workaround is usually a problem
you invented for them.

---

## The evidence ladder

Same shape as the enforcement ladder, same reasoning: the rung says who could
still be wrong.

| Rung | Evidence | Who could still be wrong |
|---|---|---|
| `revenue` | someone paid | nobody |
| `commitment` | LOI, deposit, signed pilot | they sign but never renew |
| `behaviour` | observed doing the workaround today | you, about whether yours is better |
| `stated` | they said they would | everyone - people lie politely |
| `analogue` | a comparable market did this | you, about the analogy |
| `desk` | market size, competitors, regulation | you, about everything human |
| `none` | assumed | you |

**Most validation stops at `stated`, and `stated` is near-worthless.** "Would you
use this?" always returns yes. It is the rung that feels like research and is not.

`desk` and `analogue` are cheap and they only ever *kill*, never confirm. Use them
first for exactly that.

---

## Method: test the mechanism, link by link

A hypothesis is not a conclusion. It is a **chain of because**, and every link is
separately checkable.

> A clearing hub helps independent pharmacies **because** they carry a
> reconciliation burden, **because** they settle against many payers, **which** a
> shared hub removes.

```
1. they have a reconciliation burden           → do they? how costly?
2. it comes from many-payer settlement         → or from something else?
3. a shared clearing hub removes it            → does it, or just move it?
4. therefore a clearing hub helps              → follows only if 1-3 hold
```

State it this way and the untestable version disappears. "Clearing hubs are the
future of independent retail" has no links. "They have a reconciliation burden
caused by X" has four, each answerable.

### Work a frontier, not a checklist

You rarely know the links up front, and checking one often shows it was the wrong
question. So the chain is the structure; a frontier search is how you fill it in.

Keep two sets: **open leads** and **followed leads**. Each followed lead returns
one of three things:

| Result | Means |
|---|---|
| **settles** a link | holds or breaks, at a stated evidence rung |
| **spawns** leads | the answer raised a better question |
| **both** | the common case |

Take from the frontier **cheapest first**. Independent leads run in parallel;
a dependent lead waits for its parent to settle. A lead that costs a pilot does
not get followed while a lead that costs an afternoon is still open.

Structural and regulatory leads are usually cheapest and break most often. *Can a
licensed pharmacy legally route claims through a third party here?* is a day of
desk work, and if it breaks, everything downstream is moot.

### A break is not a stop

Most breaks do not end the search. Classify one before you act on it:

| Break | Means | Do |
|---|---|---|
| **terminal** | the space is dead: illegal everywhere, no buyer exists at all | stop |
| **mechanism** | this *why* is wrong, the pain may still be real | **re-chain and continue** |
| **scope** | right idea, wrong segment or buyer | **re-target and continue** |

Only a terminal break stops the search. A mechanism break means you have learned
what the pain is *not*, which is exactly the input for a better chain: build one
from what the search just surfaced and keep spending the budget on it.

Stopping at a mechanism break is the most common failure of this method. It is
analytically correct and strategically useless. You have paid for the research and
thrown away the part that was worth having.

### Blocked is not a break

A lead can fail to settle for two entirely different reasons, and they are not
the same finding:

| Status | Means | Do |
|---|---|---|
| **break** | the evidence says this *why* is wrong | re-chain, per the table above |
| **unchecked** | not yet tried - budget or frontier ran out first | say so, name the cost |
| **blocked** | the tool needed to check it was denied or unavailable | **stop and escalate - do not write the report** |

A `WebSearch` or `WebFetch` call refused for lack of permission has told you
nothing about the market. Recording it as `unchecked` and writing the full
report anyway is how a single blocked door gets reported as five independent
findings, one per lead that hit it. If every open lead in this pass is blocked
on the same cause, stop immediately and say exactly that - one line, not a
document - so it can be unblocked and re-run, rather than spending the rest of
the budget restating the same wall from different angles.

Only write the full report once at least one lead actually ran.

### Stopping

Stop on a terminal break, when the frontier empties, or when the budget is spent.

**Never stop quietly.** An unfollowed lead is a finding: say what it was and what
it would have cost. Silence about an unchecked lead reads as "checked" -
silence about a *blocked* one reads as "tested and it failed," which is worse.

### A break is a signpost, not a tombstone

> Chains already run a shared clearing hub.

That does not end the idea. It is a durable fact about the market, and it points
in two directions at once: clearing may be solved while the reconciliation on top
of it is still manual, or independents may lack what chains have.

The reason a link broke outlives the hypothesis that exposed it and applies to
every future idea in the space. That is the part worth keeping.

### Why not "prove it right"

Supporting evidence is findable for any idea, which is what makes it worthless as
a signal. Testing a link honestly means trying to break it, and a link that
survives a real attempt is worth more than a hundred confirmations.

---

## Reporting

**Do not report the chain.** The chain is working notes. The reader wants a
decision and the facts behind it, not a walk through how you got there.

Write a document, not a trace. Sentence-case headings, a recommendation that
stands alone at the top, tables where the content is tabular, prose where it is
not. Never name a section after a step of this method.

```markdown
# <the claim, restated in one line>

**<Recommendation. Two sentences, standing alone. What to do, and roughly
what it costs. Someone who reads only this should be able to act.>**

## Where this points

The strongest chain still standing, in prose, and what would settle it. This is
the section the reader acts on, so it goes first. If the space is genuinely dead,
one line saying so and move on.

## Why the original premise fails

One paragraph. It is history the moment it is written.

## What we now know

| Fact | Evidence | Rung |
|---|---|---|
| one line, about the market and not the idea | what showed it | `desk` |

## Not checked

| Lead | Would cost | Why it matters |
|---|---|---|

## Sources

- [Source name](https://full-url) - what it established
```

Three rules that keep it readable:

**Rungs live in a column, never inline in prose.** "Rung: desk" mid-sentence is
working notes leaking into a document.

**One fact per row.** A row holding three findings is a paragraph in a table
costume, and nobody scans it.

**Every source is a clickable link to the page that actually says it.** A bare
name is a claim about a source, not a source: the reader cannot check it and has
to take your word. Link the specific page, not the site's front door.

Where no URL exists, say what it was and why it cannot be linked, in place of the
link: a phone call, a paywalled deck, a document behind a login. That is honest
and still checkable by someone who can reach it. An unlinkable source silently
listed among linked ones borrows their credibility.

Two sections carry the value. **Where this points** is what gets acted on: if a
mechanism break sent you back to re-chain, the survivor belongs there, tested as
far as budget allowed, not parked as a suggestion. **What we now know** is what
pays later, because it is about the market rather than the idea, and it is still
worth reading when the idea is forgotten.

A process oriented around *killing ideas* fails for a different reason: nobody
volunteers their idea for execution, so the tests stop being run. Locating the
break keeps the work and keeps people willing to look.

---

## Who validates

Whoever proposes an idea is the wrong party to test it - the same separation of
duties that stops QA fixing what it finds. Not because they are dishonest, but
because they wrote the chain, so the weak link is invisible to them by
construction. If they could see it, they would have fixed it already.

Validation runs from a **fresh context**: it inherits none of the accumulated
case, and it is judged on whether it located a break, not on whether the idea
survived. It is answered with **evidence, not argument** - "but if we position it
differently" does not repair a broken link, it proposes a new chain, which needs
testing like any other.

Where a team shares one lens, this is not optional. A room with one perspective
mistakes agreement for evidence.

---

## Finding problems in the first place

Standing watches beat research sprints. A sprint returns what was true the week
you looked; a watch accumulates.

Signals that indicate real pain, roughly by reliability:

- **Job postings** - companies hire against problems they have. Repeated postings
  for a manual role is a process nobody has automated.
- **Workaround artifacts** - spreadsheet templates traded in forums, third-party
  scripts, "how do you handle X" threads.
- **Complaint volume** - review sites, support forums, trade press.
- **Forced change** - a regulatory deadline or a platform deprecation creates
  budget where none existed.
- **Switching noise** - people publicly leaving an incumbent.

None of this replaces talking to five people who have the problem. That remains
the highest-value evidence available and it is not automatable - which is exactly
why it keeps getting skipped.

---

## The gate

> **No `SPEC` without a `HYPOTHESIS` at `behaviour` or above.**

Not a suggestion. It is the structural substitute for an instinct nobody may
have, and the one thing here that changes what actually gets built.

Below `behaviour`, the only permitted output is a cheaper experiment. Building is
not a way to find out.

Record the rung on every claim. A hypothesis that has sat at `desk` for a month is
not being validated - it is being avoided.
