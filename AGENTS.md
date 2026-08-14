# Knowledge & Audit Framework

AGENTS.md is the index - short rules inline, deep-dives in the linked docs. Read the linked doc whenever the area applies.

The reusable knowledge and audit framework. Consumed by sibling project repos;
contains no application code.

## Constraints

- **A constraint states its mechanism and cost**, not its birthplace. A rule you
  cannot explain is folklore, and folklore is what this framework exists to
  eliminate. `check: every entry explains why`
- **A constraint needs an enforcement binding** from the ladder
  (`compile` › `db` › `guard` › `codegen` › `test` › `gate` › `check` › `none`),
  no exceptions - including these, and the Coverage number below is derived
  from them rather than maintained beside them. `gate: validate:constraint-bindings`
- **No project names anywhere.** No repo names, no service names, no file paths
  from a consuming project. This gets dropped into new repos; another project's
  trivia is dead weight there. Naming a *tool* is fine and often necessary - a
  constraint has to be actionable. `check: grep for consuming-project identifiers`
- **Scaffold files are worked examples, never blank skeletons.** A placeholder
  gets copied verbatim and becomes the first thing that rots; an example gets
  edited down. Every scaffold line must be one an author would otherwise have to
  invent. `check: read scaffold/ end to end before adding`
- **Adoption work is not framework content.** Per-project migration plans and
  live findings belong in that project's register, never here.
  `check: no consuming-project worklists present`
- **A department's procedure ships before its entry point, never after.** A
  `/hr:` name resolving to nothing is a documented command that does not work,
  which the portable rules already call worse than an undocumented one. An
  unbuilt department is listed without a command name so the empty seat stays
  visible. `gate: validate:commands`
- **Every relative link resolves.** A framework whose own cross-references rot
  teaches the habit it exists to prevent, and a dangling path is invisible until
  someone follows it. Links inside fenced blocks are templates for other repos
  and are exempt. `gate: validate:links`
- **The manifests validate and agree on version.** A plugin that fails its own
  validator does not install, and version drift between `plugin.json` and
  `marketplace.json` installs the wrong thing silently. Found by `impact`: this
  gate existed before the constraint it enforces did. `gate: validate:manifest`
- **Every artifact is an in-toto Statement carrying a known predicate.** Prose
  describes a contract; a schema is one. The envelope is enforced rather than
  intended because "in the shape of in-toto" decays into "incompatible with it"
  through changes that each look harmless - see
  [`contracts/README.md`](contracts/README.md). `gate: validate:contracts`
- **The contract validator is proven to reject.** A validator nobody has watched
  fail is a validator nobody knows works, and every counterexample is a natural
  thing to write. `gate: test:contracts`
- **No agent ever holds the signing key.** The platform attests, not the thing
  being attested - an agent that can sign its own verdict has been separated
  from nothing. `hr sign` reads the key from the environment only, and the
  environment is CI. `gate: test:attestation`
- **An artifact that cannot be verified is refused, not warned about.** A
  warning inside a green build is a warning nobody reads. Unsigned, unknown
  keyid, tampered payload and not-a-Statement are one answer: no.
  `gate: validate:attestation`
- **The ledger is append-only in fact, not by convention.** Each entry carries
  the hash of the one before it, so an edit breaks every link after it and the
  gate names the first. A log that can be quietly rewritten is a log that will
  be. `gate: validate:ledger`
- **An entry that invoked a model records what that model was asked.** The
  prompt by digest, the tool grant it ran under, which harness ran it, and the
  procedure pinned to its content. Cost proves an agent ran; without this the
  ledger cannot say what it was told to do or what it was allowed to touch,
  and editing a procedure silently rewrites what every past entry meant.
  `gate: validate:ledger`
- **Text from outside is data, and an artifact built on it says so.** Content
  a model read from the world reaches the next model fenced and labelled, in
  a fence that content cannot close, and the entry records that the work
  rested on material hr did not author. An evidence chain that cannot tell
  outside material from first-hand work overstates every claim built on it.
  `test: lead output is fenced, and the fence cannot be closed from inside`
- **A seat runs under the grant it declared, argument patterns and denials
  included.** `Bash(hr emit *)` and `Bash` are different authorities, and
  passing the second for the first hands over a shell in place of a command.
  A harness that cannot enforce a denial must refuse the invocation rather
  than run it wider than asked.
  `guard: Select wraps every harness so an unenforceable grant refuses`
- **Durable state has one definition and two providers, not two
  implementations.** The ledger and the artifact store are reachable through
  `internal/persistence`; a consumer that names a concrete backend cannot be
  pointed at the other one, and backends tested only apart drift until they
  disagree about what they store. One suite runs against both.
  `test: file and postgres pass the same provider suite`
- **An agent is confined by something it does not have to cooperate with.**
  The tool grant needs the harness to honour a matcher and the policy guard
  needs it to run a hook; both are the harness deciding to be governed. The
  kernel holding a Landlock ruleset is not. A provider that cannot confine
  reports its own name, so an unconfined deployment is a stated fact rather
  than a control assumed to exist.
  `guard: hr confine applies the ruleset or refuses to run`
- **Policy decides at the tool call, not only at the step.** A step approved
  as one unit contains actions that are not uniform, and an approval that
  fires once is a static tool grant doing the real constraining. The engine
  runs against each call, scores it on what that call can do, treats
  `escalate` as denial because there is no human to escalate to mid-run, and
  fails closed when it cannot decide.
  `guard: hr guard runs per tool call and denies on any error`
- **The actor on an entry is the caller the token resolved to.** A client
  that can name its own actor can attribute its work to anyone, and a log
  whose attribution is self-asserted cannot answer the first question an
  incident asks.
  `test: a forged actor in the request body never reaches the ledger`
- **An identity acts only on the departments it was granted.** Authentication
  says who is calling; it never says what they may run. An absent grant is no
  departments, so forgetting to state a scope under-grants rather than quietly
  handing over the whole system.
  `test: an ungranted department is refused at both doors`
- **Risk, reversibility, autonomy, confidence and observability stay separate.**
  One tier standing in for five is how a one-line change with an irreversible
  side effect gets waved through as low-risk. A policy rule may lower the
  autonomy ceiling and never raise it, so adding a rule can only make the system
  more cautious. `gate: test:authority`
- **Memory is recalled from what is stored, never approximated.** A plausible
  answer that was never stored cannot be told apart from a real one afterwards,
  so recall matches the stored vocabulary and refuses when nothing matches. Every
  recalled fact cites its source and confidence. `gate: validate:memory`
- **A failed write never shrinks the store, and nothing is marked done unless it
  landed.** An extraction that silently failed writes nothing, and writing
  nothing over something is how a memory base disappears in one bad run.
  `gate: validate:memory`
- **Contradiction is resolved by superseding, never by overwriting.** The old
  fact is why an old decision looked right at the time, and a deletion leaves a
  tombstone with its reason. `gate: validate:memory`
- **A goal states a budget, and the check happens before work starts.** The only
  moment "this costs more than it is worth" is actionable is at admission; on the
  invoice it is a post-mortem. An unbudgeted goal wins every prioritisation by
  default because nothing can be weighed against it.
  `test: an unaffordable step never reaches policy or the model`
- **Cost is reported per accepted outcome, never as raw tokens.** Tokens are
  telemetry. Spend can fall while people do more of the work, which is not an
  improvement - which is why human touches is reported beside it.
  `test: spend is reported per accepted outcome, and undenominated when nothing was accepted`
- **The dispatcher asks the cheap question first: budget, then policy, then the
  model.** Asking an expensive question after an unaffordable answer is waste,
  and a seat that consults policy after acting has consulted nothing.
  `test: budget refusal short-circuits before policy is evaluated`
- **Acting is opt-in.** `hr run` prints its plan and stops unless
  `--execute` is passed. A dispatcher that acts by default is one bad loop away
  from an invoice, which is the overshoot the budget check exists to prevent.
  `test: a dry run decides, records, and invokes nothing`
- **Every event names the identity that caused it.** An event recorded as
  `unattributed` cannot answer the first question an incident asks, and cost
  with no actor and no goal cannot be divided by anything.
  `gate: validate:ledger`

## Coverage

`compile` 0 · `db` 0 · `guard` 3 · `codegen` 0 · `test` 8 · `gate` 15 · `check` 4 · `none` 0 - **at gate or above: 87%**

Ten gates run in CI: `validate:commands`, `validate:links`, `validate:manifest`,
`validate:contracts`, `test:contracts`, `test:attestation`,
`validate:attestation`, `validate:ledger`, `test:authority`, `validate:memory`,
`validate:constraint-bindings`.

That last one closes `DUR-001` in part: the number above is now **derived** from
the constraints and checked against what this section states, rather than being
a claim about the file that nobody could verify. It parses every `AGENTS.md` git
knows about - a derived target set, never a list - and serves both conventions:
a `## Constraints` section where every bullet must bind, and topical sections
where only bound bullets count.

The `check:` constraints share one weakness `impact` reports on every run:
their bindings name no script, so they have no derivable path scope and apply
repo-wide. A constraint that applies everywhere discriminates nowhere. That is
the standing backlog, and it is why the coverage number is the health metric.

## Documentation

Read on demand - do not pre-load.

- [`method/enforcement-ladder.md`](method/enforcement-ladder.md) - the binding vocabulary everything else references
- [`method/knowledge-layers.md`](method/knowledge-layers.md) - the four layers and layer assignment
- [`method/audit-loop.md`](method/audit-loop.md) - the audit loop that feeds them
- [`method/agent-context.md`](method/agent-context.md) - context budgets, the pointer pattern, H exclusion
- [`departments/README.md`](departments/README.md) - the six departments, their handoff schemas, and the four ladders
- [`departments/engineering.md`](departments/engineering.md) - read when touching intake, `CHANGE`, `DESIGN`, or anything that decides what to build. Covers: the two queues, the intake frontier, the reversibility ladder, design as an O diff
- [`departments/product.md`](departments/product.md) - read when touching `SPEC`, acceptance criteria, or anything that decides what done means. Covers: what makes a criterion executable, out-of-scope as half the artifact, serving one link of a hypothesis
- [`departments/release.md`](departments/release.md) - read when touching `RELEASE`, rollout, or a ship decision. Covers: the rehearsed-rollback gate, consequence classes as an axis separate from reversibility
- [`departments/ops.md`](departments/ops.md) - read when touching alerts, `SIGNAL`, or a failure mode's rung. Covers: the observability ladder, declared alert coverage, why an agent never sits in the paging path
- [`contracts/README.md`](contracts/README.md) - read when touching an artifact shape or signing. Covers: the in-toto envelope, why the shape is enforced, DSSE and who holds the key
- [`constraints/portable.md`](constraints/portable.md) - cross-project rules
- `scaffold/` - worked examples copied into new projects
