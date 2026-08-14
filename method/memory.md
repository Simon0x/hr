# Memory

What the system learned that **is not in any file**.

A map of the codebase can be rebuilt by re-reading the codebase. "We tried this
and it failed because X", "this gate is flaky on a cold cache", "that constraint
was proposed and rejected" — none of those are recoverable from anything on
disk. Lose them and they are gone, and the work that produced them gets paid for
again.

That is the whole boundary. If re-running a tool would reproduce the fact, it is
not memory; it is an index, and it belongs wherever it can be regenerated.

---

## The failure this prevents

> An agent that retrieved results from an earlier bad run reused them with **more**
> confidence than the first time, because memory had given the wrong answer the
> appearance of established precedent.

That is the same mechanism the H layer excludes frozen records for, arriving from
the other direction. Memory does not decay gracefully on its own: a wrong fact
sitting in a store gets more trusted with age, not less, because age reads as
corroboration.

Everything below exists to stop that.

## Recall is grounded, and refuses

Retrieval matches the question against the vocabulary that is **actually
stored** — the words appearing in real memories — and nothing else. No
synonyms, no near-matches, nothing reached for from a model's training.

If no stored word matches the question, recall says so and stops.

That refusal is the feature. The dangerous failure of a memory system is not
returning nothing; it is returning something plausible that was never stored,
because afterwards nobody can tell the two apart. A gap announces itself. A
confabulation does not.

Every hit carries its source and its confidence, so a recalled fact can be
re-checked when it is doubted rather than argued about.

| Confidence | Means |
|---|---|
| `observed` | seen directly, with evidence that resolves |
| `inferred` | concluded from things that were observed |
| `unverified` | asserted and not yet checked — never served as though observed |

## Staleness is a provenance problem, not a schedule problem

`lastVerified` is a different field from `learnedAt`, and it is the one that
makes staleness detectable at all. A fact recorded in January and never
re-checked is not the same as a fact confirmed last week, and a store that
records only when it *learned* something cannot tell them apart.

A memory past its `verifyEvery` interval is **withheld with the reason**, never
silently dropped and never quietly served. It may well still be true, and hiding
it entirely is its own kind of lie.

Staleness is reported, never a build failure. Failing on it would teach people
to set the interval to never, which converts a visible problem into an invisible
one.

## Two clocks, because they disagree

`learnedAt` is when we came to believe it. `trueFrom` / `trueUntil` are when the
fact itself held. Those diverge constantly — you find out in August that
something stopped being true in June — and collapsing them makes *"which of
these two contradicting memories is current"* unanswerable.

A fact that stopped being true gets `trueUntil` rather than deletion. It is the
reason an old decision looked right at the time, and removing it makes the
historical record incoherent.

## Contradiction is resolved by superseding

Never by overwriting. A new memory names the one it replaces; the old one stops
being served and stays on disk. A supersede naming something that is not in the
store is refused — otherwise the contradiction it claims to resolve is still
open, and both facts read as current.

## Quarantine

Anything from an untrusted source — retrieved web content, an unreviewed
proposal, output nobody checked — is stored **quarantined**. It is never served
as fact until a human reviews it, and it can never be promoted into a durable
layer while quarantined.

This is the memory-poisoning boundary. Memory is writable at runtime and
persists across sessions, which makes it the highest-value thing to attack: poison
it once and every later action carries the attacker's intent forward, wearing the
authority of something the system believes it learned.

## Forgetting

The most underrated operation. Entries that are wrong, stale, or were never
relevant accumulate quietly and add noise to every future recall.

A deletion needs a reason and leaves a tombstone. A memory that vanishes without
trace is indistinguishable from one that was never stored, and the difference
matters the moment someone asks why the system stopped knowing something.

## Three rules that stop the store rotting

Learned the hard way, and worth stating as rules rather than rediscovering:

1. **A failed write never shrinks the store.** An extraction that silently
   failed produces nothing, and writing nothing over something is how a memory
   base disappears in a single bad run. If the guard refuses, do not force past
   it — find out why it was empty.
2. **Nothing is marked done unless it landed.** A queued item whose write failed
   must stay queued. Marking it complete loses the content permanently, with
   nothing to indicate anything went missing.
3. **Deletions prune.** When the thing a memory is about disappears, the memory
   goes too, or the store fills with confident facts about things that no longer
   exist.

## Promotion is the goal

A memory recalled over and over should stop being a memory. The audit loop
already asks the promotion question at closure; this is the same question with a
store behind it.

| Recalled repeatedly | Belongs in |
|---|---|
| a rule that must not break again | **C**, with a binding |
| how the system currently works | **O** |
| a problem still open | **R** |

`promotedTo` records where it went. A store whose entries never get promoted is
not accumulating knowledge — it is accumulating notes.
