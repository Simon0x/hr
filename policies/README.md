# Policies

Policy as **data**, evaluated by `hr authority`
(implementation: [`../internal/policy/authority.go`](../internal/policy/authority.go)).
Not as instructions in a prompt: an agent that reads its own limits can
reinterpret them, and a convincing argument is not an authorisation.

## Five dimensions, and why not one

The reversibility ladder answers *what does it cost to undo*. That is one
question, and the framework used to let it stand in for four others.

| Dimension | Answers | Scale |
|---|---|---|
| risk | what it costs to be wrong | `R0`-`R4` |
| reversibility | what it costs to undo | `revert` … `destroy` |
| autonomy | what may happen without a human | `A0`-`A4` |
| confidence | how good the evidence is | `certain` \| `likely` \| `unknown` |
| observability | how fast harm is noticed | `refuses` … `silent` |

They disagree on purpose, and the disagreements are the useful part:

- A **one-line change that sends mail** is `revert` on reversibility and `R2` or
  worse on consequence. The commit comes back; the mail does not.
- An **internal migration nobody outside can see** is `migrate` — expensive to
  undo — and `R1`. Expensive is not the same as dangerous.
- A **well-tested action with no monitoring** has good confidence and `silent`
  observability. Nobody finding out is not the same as nothing going wrong.

**The strictest applicable rule governs.** A rule may lower the autonomy ceiling
and never raise it, so adding a rule can only ever make the system more
cautious - which is the property that makes policy safe to edit.

## Verdicts are three, not two

| Verdict | Exit | Means |
|---|---|---|
| `allow` | 0 | proceed within the grant |
| `refuse` | 1 | no |
| `escalate` | 3 | not without a human |

`refuse` and `escalate` must not share an exit code. One says the action is
wrong; the other says the action may well be right and exceeds what may happen
unattended. Collapsing them turns every escalation into a dead end and teaches
people to route around the engine.

## Every decision names its policy by digest

A `DECISION` carries the hash of the policy that produced it, not its name. A
policy edited afterwards must not silently re-describe a decision made under the
old one - which is the same reason artifacts reference by digest and never by a
mutable tag.

## What this does not do yet

It decides; it does not **enforce**. Nothing currently requires a caller to
consult `authority` before acting, so the engine is advisory in exactly the way
the review warns about - "policy expressed only as instructions that an agent
may ignore".

Closing that means the enforcement point moving to the tool-calling layer, where
the agent cannot route around it. That is the same lesson OPA and Cedar landed
on for agent authorisation: the agent does not decide what is allowed, the
policy engine does, and it sits in the path rather than beside it.

Recorded as `SEC-003` in the register rather than left to be discovered.
