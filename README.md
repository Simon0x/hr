# hr - Harness Runtime

A departmental operating model: a reusable way to hold what a project knows, so
that checking it gets cheaper each time instead of more expensive.

Yes, it reads as Human Resources. It manages your departments, so the collision
is the joke and the joke is accurate. Skills are namespaced `/hr:qa`,
`/hr:discovery`, `/hr:engineering`, `/hr:audit`.

It exists for a failure mode that shows up in any long-running codebase. The work
settles into a rhythm - **audit → findings ledger → batched fixes → closure
report** - and every cycle pays full price, because nothing from the last one was
kept in a form the next one could use. The same check gets re-run by hand.
Validation scripts accumulate without ever reaching CI. Hard-won knowledge sits
in documents that quietly stopped being true.

That is not a discipline problem. It is a **structure** problem: there is no
defined place for a lesson to land, so lessons land wherever the work happened to
be, and rot there.

## Setup

### Claude Code

```
/plugin marketplace add Simon0x/hr
/plugin install hr@hr
```

That is it. You now have `/hr:qa`, `/hr:discovery`, `/hr:engineering` and
`/hr:audit` in every project. Pull later changes with `/plugin marketplace
update`.

The repo is public, so the marketplace command needs no credentials and no extra
setup. Claude Code manages the clone and keeps it updated in the background.

**Working on hr itself:** `claude --plugin-dir /path/to/hr` loads a local copy and
overrides the installed version for that session, so you can edit and test
without uninstalling. `SKILL.md` edits apply immediately; run `/reload-plugins`
after changing anything in `agents/`.

### Any other agent, or none

Nothing here requires Claude Code. The procedures in `departments/` are plain
markdown: point any agent at one, or read it and follow it yourself. Only
`skills/` and `agents/` are Claude-specific, and they hold tool grants and
pointers, never logic.

### On a project

1. Copy `scaffold/AGENTS.md` to the repo root and edit it down.
2. Copy `scaffold/REGISTER.md` to `docs/audit/REGISTER.md`.
3. Merge `scaffold/settings.json` into `.claude/settings.json`.
4. Inherit `constraints/portable.md`. Do not copy it: layers may only tighten.

## Layout

```
method/       how knowledge is held and kept true
departments/  who does what, and the contracts between them
skills/       /hr:* entry points (Claude Code)
agents/       tool grants (Claude Code)
constraints/  rules that carry across projects
contracts/    the handoff schemas, executable
scaffold/     worked examples to copy into a new repo
cmd/hr/       the hr binary - one Go program, every subcommand
internal/     hr's implementation - policy, ledger, contracts, workflow, daemon
trust/        public signing keys; the private half lives only in CI
```

`hr` is a single static Go binary - `go build -o hr ./cmd/hr`, no runtime install
step. Skills build it on demand if it is not already present.

| Doc | What it gives you |
|---|---|
| [`method/enforcement-ladder.md`](method/enforcement-ladder.md) | **Start here.** How a constraint declares its enforcement, and why the rung matters |
| [`method/knowledge-layers.md`](method/knowledge-layers.md) | The four layers, their decay profiles, and how to assign a fact to one |
| [`method/audit-loop.md`](method/audit-loop.md) | The audit that consumes and feeds those layers |
| [`method/agent-context.md`](method/agent-context.md) | Context budgets, the pointer pattern, keeping frozen records out of context |
| [`departments/README.md`](departments/README.md) | The six departments, handoff schemas, CI versus judgment |
| [`departments/product.md`](departments/product.md) | Product: executable criteria, out-of-scope, which link a spec serves |
| [`departments/engineering.md`](departments/engineering.md) | Engineering: the intake questions, the reversibility ladder, design as an O diff |
| [`departments/qa.md`](departments/qa.md) | QA: acceptance, blast radius, the verdict schema |
| [`departments/release.md`](departments/release.md) | Release: rehearsed rollback, consequence versus reversibility |
| [`departments/ops.md`](departments/ops.md) | Ops: the observability ladder, alert coverage, signal → finding |
| [`COMMANDS.md`](COMMANDS.md) | The entry point per department, and how `hr`, procedures and tool shims layer |
| [`constraints/portable.md`](constraints/portable.md) | Cross-project rules, with bindings |
| [`scaffold/README.md`](scaffold/README.md) | What to copy, and what to keep when you edit it down |

## The idea

Two things. The second is what makes the first hold.

### Departments are contracts

Discovery decides whether to build. Product turns that into criteria.
Engineering decides what the change should be and makes it. QA proves. Release
ships. Ops watches and feeds back.

Each consumes a typed artifact and emits one:

```
SIGNAL → PROBLEM → HYPOTHESIS → SPEC → CHANGE → VERDICT → RELEASE → SIGNAL
```

The arrows are schemas, not conversations. That is the whole difference between
an operating model and six chatbots. A department is a contract, never a person
or an agent, so you can start with a human following it and swap in an agent
later without redesigning anything.

### Every claim declares how strongly it is held

Same discipline everywhere, different ladder per department:

| Claim | Ladder, strongest first |
|---|---|
| This rule cannot be broken | `compile` `db` `guard` `codegen` `test` `gate` `check` `none` |
| This is true of the market | `revenue` `commitment` `behaviour` `stated` `analogue` `desk` `none` |
| This change can be undone | `revert` `flag` `expand` `migrate` `emit` `destroy` |
| This breakage gets noticed | `refuses` `alerts` `dashboards` `logs` `silent` |

The rung is the most useful fact about any claim, because it names **who could
still be wrong** - or, on the last two, who gets hurt and who finds out. Nobody,
anyone who skips CI, anyone in a hurry, or everyone.

```markdown
- Money is integer cents, never float.   `compile`   nobody can break it
- QA has no Edit tool.                   `compile`   authority is a tool grant
- Independent pharmacies will pay.       `stated`    nobody has paid yet
- Ships the new pricing path.            `flag`      dark until a tenant opts in
- The payment webhook stops arriving.    `silent`    the customer tells us
```

Five claims, one standard. A rule nobody can break, an org boundary the
harness enforces rather than a policy asking nicely, a market claim resting on
the rung that feels like research and is not, a change that costs one flag flip
to undo, and a failure mode that will be discovered by the person it hurts.

The last one is legal. Undeclared, it is not.

**`want:`** marks a claim that could climb but has not. That is the backlog that
pays, and the closing question of every audit is *"can this go up a rung?"*

## Scope

Written for internal use and published in the open under [MIT](LICENSE) - take
it, edit it down, no attribution ceremony beyond keeping the notice.

The method and departments are stack-agnostic; only `constraints/portable.md` and
the scaffold examples carry stack assumptions, and both are meant to be edited
down.
