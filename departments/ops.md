# Ops

Owns **noticing**. Consumes `RELEASE`, emits `SIGNAL`.

Ops holds `Read` and `Bash(logs, metrics)` and writes nothing. That is
deliberate, and it has a consequence people miss: **Ops cannot instrument
anything.** It can only observe what Engineering already built. An Ops seat with
no instrumentation to read is an empty seat that looks staffed - the same failure
as an unfilled Discovery seat, which does not stop work but lets work start
without evidence, silently.

So the instrumentation is Engineering's output, specified at design time in
`failure-modes[]`, and this document is the contract it is written against.

---

## The observability ladder

Same shape as every other ladder here. The rung says who finds out first.

| Rung | Means | Who finds out first |
|---|---|---|
| `refuses` | the system will not start, or will not accept the bad state | nobody - it never ran |
| `alerts` | a page fires on a symptom a user feels | you, before the user reports it |
| `dashboards` | a metric exists and moves | you, if you happen to look |
| `logs` | recorded and searchable afterwards | you, during the postmortem |
| `silent` | no signal at all | the customer |

`refuses` outranks `alerts` for the same reason `compile` outranks `gate`: a
failure that cannot occur needs nobody watching for it. A persistence layer that
throws at startup rather than handing back a map, a process that crashes on an
unreachable database rather than surviving its own outage - those are top-rung
observability, and they are already written down in the portable constraints
under Durability. The ladder is what makes them one idea instead of five.

`silent` is a declared blind spot, exactly like `none`. Honest beats absent.

**The climb question applies here too:** most incidents cannot be prevented, but
nearly all of them can be detected one rung faster.

---

## Alert on symptoms, not causes

Page on what a user feels - error rate, checkout failures, latency at the
percentile that matters. CPU at 90% is a dashboard.

This is not a style preference. A cause-shaped alert dies at the next refactor,
because the cause moved and the alert now watches a code path nobody executes. A
symptom-shaped alert survives the rewrite, because the symptom is defined in the
user's terms and those do not change when the implementation does.

The corollary is that a health endpoint returning 200 while the database is
unreachable is worse than no health endpoint. It converts an outage into a
*silent* outage and it satisfies every check you built.

---

## Coverage is declared, not curated

Each alert declares what it covers, in its own definition, the way a gate script
declares `@covers`:

```
# @covers services/*/checkout/**
```

The tool builds the map at run time and intersects it with a diff or a path set.
Nobody maintains an inventory.

A hand-written failure-mode → alert list is a hand-maintained document and rots
like one, which is the same reason `guard` outranks `gate` on the enforcement
ladder. The map that matters is the one nobody has to remember to update.

Output, mirroring blast radius:

```
watched      path → alerts that cover it → firing/quiet
unwatched    path → nothing watches this in production   ← the finding
orphan       alert whose covered paths no longer exist   (delete it)
```

`unwatched` is the standing backlog. `orphan` is the decay signal - an alert
pointing at deleted code is indistinguishable from working coverage until the
day it matters.

---

## Three rules that keep alerts alive

- **Every alert pages, or it does not exist.** Anything that merely wants
  attention later is a `SIGNAL` into the register, not a message in a channel.
  Channels that accumulate unactioned alerts are where alerting goes to die, and
  the death is not local: an ignored alert trains the ignoring of every alert
  beside it.
- **An alert with no runbook is a notification.** The runbook is present-tense
  operational truth, so it is O orientation, and the alert links to it. If nobody can state
  what to do when it fires, nobody will do anything when it fires.
- **An alert that fired and was correct to ignore is a bug in the alert.** Fix
  the threshold or delete it. Tolerating it costs more than the outage it was
  built for, because it spends the one budget that does not refill: the
  willingness to look.

---

## Where an agent belongs, and where it must not

The framework's rule holds here without amendment - deterministic where a script
does better, agents where a script cannot go.

- **A script decides whether to page.** An agent must never sit in the paging
  path. That buys nondeterminism, latency and cost at the exact point where
  certainty is the product.
- **An agent triages afterwards.** Reading the day's error groups, correlating
  them against the last `RELEASE` digest, separating the new from the known, and
  writing `SIGNAL`s into the register is judgment over unfamiliar output. It is
  also the step that otherwise never happens, because it is nobody's emergency.

That correlation is worth building deliberately. `RELEASE` already carries
`artifact-digest` and `rollback-target`; errors attributed to a digest are what
make *roll back or fix forward* a decidable question instead of an argument held
under time pressure.

---

## From signal to finding

```
alert fires → incident → SIGNAL → register finding → Engineering closes it
```

`SIGNAL` carries `→ register-finding-id`, so an incident that produces no finding
produced nothing durable. That is allowed exactly once per class of incident, and
saying so out loud is the point.

Closure then asks **two** climb questions rather than one:

1. Can a gate stop this recurring? *(enforcement ladder)*
2. If it recurs anyway, will anyone find out sooner? *(observability ladder)*

The second is where the compounding lives. A postmortem that ends at "we fixed
it" has bought a fix; one that moves a failure mode from `logs` to `alerts` has
bought every future occurrence.

---

## Agent constraints

- **No write access of any kind.** Ops observes. A seat that can change the
  system it is judging will change it and report that the system is fine.
- **A signal without evidence is not a signal.** Log line, metric series, request
  id, or the digest it was attributed to.
- **Do not re-derive what the alert already asserts.** If the answer is in the
  alert's own definition, read it. Re-deriving is the cost the declaration was
  built to remove.
- **`unwatched` is a finding.** Silence about a failure mode nothing observes
  reads as observed.
