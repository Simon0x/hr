# Operating Model

A department is a **contract**, not a person or an agent. The agent is the
current implementation. That separation means you can start with a human
following the contract and swap in an agent later without redesigning anything.

## The spine

Each department consumes a typed artifact and emits one. The arrows are schemas,
not conversations - that is the difference between an operating model and five
chatbots.

| Dept | Charter | Consumes | Emits |
|---|---|---|---|
| **Discovery** | whether to build at all | `SIGNAL` | `PROBLEM`, `HYPOTHESIS` |
| **Product** | intent → testable criteria | `HYPOTHESIS`, register | `SPEC` |
| **Engineering** | criteria → code | `SPEC` **or** a finding, C, O | `CHANGE` |
| **QA** | code → verdict | `CHANGE`, `SPEC` | `VERDICT` |
| **Release** | verdict → running system | `VERDICT` | `RELEASE` |
| **Ops** | running system → signals | `RELEASE` | `SIGNAL` |

`SIGNAL` feeds back to Product as new intent, or to Engineering as a new
constraint. The audit loop is this loop restricted to one department.

Engineering is the one department with two inbound queues, and the second one -
an open register finding - is why it can run before Product exists. Most real
work is never spec'd, and asking Product for a micro-`SPEC` per dependency bump
is bureaucracy that gets skipped inside a week. See
[`engineering.md`](engineering.md).

## One discipline, four ladders

Every department holds its claims to a ladder, and the rung always answers the
same question: **who could still be wrong?**

| Ladder | Ranks | Lives in |
|---|---|---|
| enforcement | how strongly a rule is held | [`../method/enforcement-ladder.md`](../method/enforcement-ladder.md) |
| evidence | how strongly a market claim is held | [`discovery.md`](discovery.md) |
| reversibility | who is hurt if a change is wrong | [`engineering.md`](engineering.md) |
| observability | who finds out first when it breaks | [`ops.md`](ops.md) |

Three of them bottom out in an absence - `none`, `none`, `silent` - which is
permitted only when **declared**. A declared blind spot is a backlog item; an
undeclared one is a surprise with a date on it. Reversibility is the exception:
`destroy` is a choice rather than an absence, and it is the rung that has to be
argued for rather than merely admitted.

Each ladder carries the same closing question: **can this go up a rung?**

## What is deterministic and what is judgment

The enforcement ladder decides. It is the same question in both places.

| Rung | Runs where | Who invokes |
|---|---|---|
| `compile` `db` `guard` `codegen` `test` `gate` | CI, every push | nobody - automatic |
| `check` | an agent, deliberately | a human wearing the department hat |
| `none` | flagged at audit | a human |

**No LLM in CI.** If something is worth checking on every push, build its gate.
Pointing a model at a diff every push buys nondeterminism, cost, and a check that
cannot be reasoned about - for work a script does better. Agents earn their place
exactly where a script cannot go: judging intent, weighing risk, reading a spec.

## Authority is a tool grant

Departmental boundaries are enforced by what each agent can touch, not by
instructions telling it to behave.

| Dept | Gets | Deliberately lacks |
|---|---|---|
| Discovery (propose) | Read, Write(hypotheses), WebSearch | **the kill verdict** |
| Discovery (kill) | Read, WebSearch | **Write** - it argues, it does not author |
| Product | Read, Write(specs) | source edit |
| Engineering (intake) | Read, Grep, Bash(impact) | **Edit** - it briefs, it does not build |
| Engineering (build) | Read, Edit, Bash | deploy, and **write to C or H** |
| QA | Read, Grep, Bash(validate:\*, test) | **Edit** |
| Release | Bash(build, deploy, rollback) | source edit |
| Ops | Read, Bash(logs, metrics) | write anything |

QA lacking `Edit` is the load-bearing one. An agent that can both find and fix
will quietly fix and report success, and you lose the finding. Separation of
duties at the `compile` rung: make the wrong action impossible, not forbidden.

Engineering lacking write access to the C layer is the second. An agent blocked by a
constraint edits the constraint, and the resulting diff looks self-consistent, so
review does not catch it. A constraint change is proposed through `DESIGN` and
resolved before the code lands.

### Judgment seats lack the tool that shortcuts the judgment

Three seats now withhold the obvious tool: the kill seat has no `Write`, QA has
no `Edit`, intake has no `Edit`. That is one rule, not three coincidences.

**A seat whose job is to think clearly about work is denied the ability to do
that work**, because doing it is always the cheaper path and it always wins under
time pressure. Apply this when adding the fourth seat, rather than rediscovering
it after the seat has quietly stopped working.

### Grants and approvals are different things

A static grant answers *what this role may ever do*. An approval prompt answers
*whether this particular action proceeds now*. Both are useful and they are not
substitutes.

Prefer the grant. It is the `compile` rung - the tool is absent, so there is
nothing to approve and nothing to fatigue past. Approvals are the `check:` rung:
a human in the loop, with a human's attention budget, which erodes.

**The harness has a field for each, and they are easy to confuse.** In skill
frontmatter, `disallowed-tools` removes a tool from the pool - that is the
grant. `allowed-tools` pre-approves a tool so it runs without prompting - that
is the approval. Reaching for `allowed-tools` to express a restriction produces
the opposite of the intent: it grants nothing and withholds nothing, while
quietly removing the prompt that was the only thing standing in the way.

| Want | Field | Rung |
|---|---|---|
| QA can never edit | `disallowed-tools: Edit` | `compile` |
| QA runs `impact` without prompting | `allowed-tools: Bash(.../impact *)` | convenience, not enforcement |

Both clear when the next message is sent, so neither survives the turn. A grant
that expires is weaker than one that does not, and the durable form is a
subagent with its own `tools:` list - which is why `agents/` exists alongside
`skills/`.

### What a tool grant does not survive

An unrestricted `Bash` reopens every door the grant closed. A seat with no
`Edit` but with `Bash` can still write a file, and the grant it appeared to hold
was never at the `compile` rung at all.

Withholding a tool is honest about *intent* and only enforces where the tool is
the only route. Where the boundary must hold against a determined process rather
than a slip, it needs the permission layer - scoped `Bash(...)` rules, path
denials, or an OS sandbox - and it needs to be said which one is load-bearing.
Recorded as `SEC-002` in the register rather than left to be discovered.

### Identity is a third axis, and an agent does not borrow one

An agent acts under **its own identity**, never a person's. The grant says what a
seat may do; the identity says who did it. Collapsing the two destroys the second
answer.

It is tempting to let an agent carry the operator's credentials - it is one line
of setup, the audit log fills in, and the work gets done. What it actually buys
is a log that cannot distinguish the agent from the human, which is precisely the
question an incident asks first. Every downstream control degrades with it:
least privilege becomes the union of everything that person may do, revocation
means disabling a colleague, and rate limits meant for one actor now cover two.

| | Wrong | Right |
|---|---|---|
| Principal | the operator's account | a distinct workload identity per seat |
| Lifetime | as long as the person's session | short-lived, scoped to the task |
| Scope | everything the person may do | the department's grant, and nothing else |
| On revoke | locks out a human | retires one agent |
| In the log | "Simon deployed" | "engineering-build, delegated by Simon, deployed" |

**Delegation is the accountable form.** A human authorizes a seat to act, the
grant is bounded and expires, and both facts appear in the record: which agent
acted, and who stood behind it. That answers accountability *and* separation of
duties, which is what the borrowed-credential arrangement only appeared to do.

Where a tool offers no identity of its own, say so and treat it as a declared
blind spot - `none` on the enforcement ladder - rather than reaching for the
operator's credentials to fill the gap.

## Handoff schemas

Minimal, and each is the input the next department needs to do its job without a
human translating.

```
PROBLEM    v, who, when, cost, workaround
HYPOTHESIS v, claim, chain[{link, status, rung, source}], break, points-to
SPEC     v, id, serves{hypothesis-id, link}, intent, acceptance[] (each executable), out-of-scope[]
CHANGE   v, serves{spec-id | finding-id}, diff, rung, constraints-touched[], routed{c[], o[], r[]}
VERDICT  v, spec-id, acceptance{met|unmet|unverifiable}[], blast-radius{}, risks[]
RELEASE  v, artifact-digest, rollback-target, gates-green[]
SIGNAL   v, source, severity, evidence, → register-finding-id
```

`CHANGE.rung` is from the reversibility ladder, and `Release` reads it: a
`RELEASE` cannot state a `rollback-target` it has not been told exists.
`routed{}` is what the change put back into the knowledge base - empty is
legitimate on a small change and suspicious on a large one.

**`DESIGN` is not in this list on purpose.** Engineering both writes and consumes
it, so it never crosses a departmental boundary and needs no shared schema
guarantee. Its shape is in [`engineering.md`](engineering.md).

**Versioned, and unknown majors fail closed.** A minor version may add optional
fields; a breaking change increments the major, and a consumer that meets a major
it does not know **refuses** rather than reading what it recognises. A Release
that half-understands a `VERDICT` is worse than one that stops - it ships on a
verdict it did not actually read.

**Artifacts reference by digest, never by a mutable tag.** A `RELEASE` names an
image digest, not `:latest`; a `VERDICT` names a commit SHA, not a branch. A
handoff whose referent can change after it is written is not a record of
anything.

## Order of adoption

Build the department that is blocking the most work, not the first in the chain.
A department can be a human following its contract on day one; the contract is
what matters, and the agent is an optimization.
