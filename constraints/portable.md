# Portable Constraints

Rules that hold across projects, not just the one they were learned in.

Every entry states the mechanism and what it costs when violated. That reasoning
is the provenance - a rule you cannot explain is folklore, and folklore is what
this framework exists to eliminate.

Each carries an enforcement binding per `../method/enforcement-ladder.md`:
`compile` › `db` › `guard` › `codegen` › `test` › `gate` › `check` › `none`.
**`want: <rung>`** marks a constraint that could climb but has not - the standing
backlog, and where audit effort converts into permanent leverage.

Project-specific rules do not belong here; they live in that repo's `AGENTS.md`.

## Layering

A project inherits this file. It does not copy it, and it does not edit it.

Its `AGENTS.md` layers on top, and **overrides are monotonic - a layer may only
tighten**:

| A project may | A project may not |
|---|---|
| add a constraint | delete an inherited one |
| raise a rung (`check:` → `guard:`) | lower a rung |
| narrow scope ("also applies to X") | broaden an exemption |
| mark one **not applicable**, with a reason | silently ignore one |

That asymmetry is what makes inheritance safe: the core can be updated without
auditing every consumer for a rule that was quietly loosened.

"Not applicable" is a real state and must be written down - a Go service does not
care about barrel-file imports. An unstated exemption is indistinguishable from a
violation, which is how a shared ruleset stops meaning anything.

Constraints reach this file by **second independent occurrence**: a rule learned
once is a project constraint, the same rule learned again in an unrelated
codebase is portable. Promotion on first sighting is how you get a shared file
full of one project's local habits.

---

## Making the dangerous path impossible

The highest-value pattern here, and the one to adopt first. Ranked by rung:

- **Wrap the dangerous handle in a type the unsafe call cannot accept.** A write
  pool that satisfies neither the query interface nor the transaction API turns
  "who wrote this row" from a review note into a build failure. Provide exactly
  two doors: the audited path, and a deliberately greppable escape hatch. `compile`
- **Back it with a database that refuses.** Roles per binary (a SELECT-only read
  role), grants declared alongside the schema, and a trigger that raises when the
  audit context is missing. The app cannot forget, because the database will not
  let it. `db`
- **Back that with a static guard whose target set is derived, not listed.**
  Derive it from schema declarations plus query files at scan time, and run it
  from one source in three places: the lint task, the pre-push hook, and CI. `guard`

A hardcoded target list rots in a month. That is the whole reason `guard`
outranks `gate`.

## Generated artifacts

- **Consumers import types only from the generated module.** No local
  redefinition of an API shape, ever. A drifted client becomes a type error
  rather than a runtime surprise. `codegen`
- Never edit generated output. Convert at the boundary instead. `codegen`
- **Codegen is a user-run command** - not CI, not an agent. It needs a service
  running on a known port. `check: stated under "Never run these"`
- Match query column order to table column order, or the generator emits
  anonymous row structs instead of model types. `check: on new queries`
- Enums are generated const objects, never string literals at the call site.
  `as const` over TypeScript `enum` - enums emit runtime artifacts and behave
  badly across module boundaries. `check: grep string literals at enum call sites`

## Identity

- **Never truncate a UUID.** A v7's leading segments are `unix_ts_ms`, identical
  for every ID minted within the same ~65 seconds, so a truncated v7 collides.
  Applies to storage *and* display - `id.slice(0, 8)` is not a short handle. Show
  a real reference field or a dash. `check: grep slice/substring on id fields` `want: guard`
- **Entities link by `id` only.** No slug or human-readable key is a lookup
  target. Before adding one, check whether anything will actually read it -
  write-only identifiers are a recurring cleanup cost. `check: review new unique columns`
- Brand and prefix domain IDs (`tnt_`, `usr_`, `ord_`). Raw-string IDs make
  cross-entity mixups type-invisible. `check: constructor functions only` `want: compile`

## Schema & migrations

- **Declarative schema is the source of truth.** Never hand-write migration SQL;
  edit the schema file and let the tool generate the diff.
  `check: review every generated migration`
- Exception: column renames. Diff tools emit DROP + ADD, which loses data and
  regresses the generated structs. Hand-write the `ALTER TABLE ... RENAME COLUMN`.
  `check: review every generated migration`
- Use database ENUMs for varchar columns with a known value set - the generator
  turns them into unions for free. `check: review new migrations`
- Never cast a **column** to text in query SQL: type-safe SQL generators lose
  NOT NULL through the cast, and most drivers already return numeric as string.
  Parameter casts are correct and expected. `check: manual audit` `want: guard`
- Never widen a generated row type with an index signature. It erases the column
  typing you just paid for. `check: manual audit` `want: guard`

## Connection pooling

- `DISCARD ALL` before returning a tenant-scoped connection to the pool. If
  `DISCARD ALL` itself fails, destroy the connection - never return a dirty one.
  `check: grep pool release paths` `want: compile`
- Inside a transaction, set tenant context with transaction-scoped settings so
  rollback cleans up automatically. Session-scoped settings outside a transaction
  require the discard above. `check: grep context-setting call sites`

## Money

- Money is an integer minor unit or an arbitrary-precision decimal. Never IEEE
  float. A project may deliberately use floats at fixed scale - that is a
  decision to write down, not a default to inherit. `none` `want: compile`
- Brand the money type. A bare `number` alias still permits `a + b`, which
  compiles and is silently wrong under any rounding discipline.
  `check: grep raw arithmetic on money fields` `want: compile`

## Security

- No fallback secrets, ever. `process.env.JWT_SECRET || 'dev-secret'` is token
  forgery with extra steps. Throw at startup when unset.
  `check: grep || adjacent to secret env reads` `want: guard`
- If fallbacks exist during a migration, exactly **one** blocked-value list
  guards them. Two dev-secret strings against a checker that knows one means a
  service passes its own startup assertion in production. `want: guard`
- **Never commit a secret to a runner file.** Taskfiles, Makefiles and compose
  files are read as configuration and reviewed as boilerplate, so a token there
  survives review indefinitely. Secrets belong in a parameter store or the
  platform's secret command. `want: guard`
- `cors({ origin: true, credentials: true })` reflects *any* origin and pairs it
  with credentials, so any site can make authenticated cross-origin requests for
  a logged-in user. Internal services need no CORS at all; the public gateway
  needs an explicit allowlist. `check: grep cors registration` `want: guard`
- Never accept an auth token from a URL query string. It lands in access logs,
  CDN logs, browser history, and `Referer`. `check: grep query-param token reads`
- Internal service-to-service auth defaults to its **strict** mode. A compat mode
  accepting a bare header means any process with network reach impersonates a
  trusted service. `check: read the mode resolver's default`
- The database is never publicly accessible. Remote access is a port-forward
  through a bastion that auto-stops when idle. `check: infra review`
- Keep a "do not regress" list in the infra repo: public accessibility, deletion
  protection, backup retention, no `0.0.0.0/0` ingress, scoped KMS grants. It is
  the checklist a reviewer actually uses. `check: the list itself`

## Durability

These are the top rung of the observability ladder in
`../departments/ops.md` - `refuses`, the state where nobody has to notice because
it cannot happen. They were each learned separately; the ladder is what makes
them one idea, and it is the thing to reach for when adding the next one.

- A persistence layer must never silently downgrade to memory. An unimplemented
  backend throws at startup; it does not warn and hand back a map. The system
  then looks healthy while every write is discarded.
  `check: grep fallback warnings` `want: guard`
- The same applies to a *configured* backend that is unreachable. Fail closed and
  crash; a process that survives its own database outage is losing data quietly.
  `want: guard`
- Every service registers a `SIGTERM` handler and closes its server. Without one
  the orchestrator sends `SIGKILL` after the grace period and in-flight work is
  lost. `check: grep signal handlers` `want: guard`
- In-memory state that must survive restart (order books, idempotency keys,
  auction phase) is event-sourced or persisted. A module-level map is a data loss
  incident with a deploy as its trigger. `check: enumerate module-level maps`
- Enqueue background jobs in the **same transaction** as the write that causes
  them, or accept a lost-job window. `check: review job enqueue sites`
- **Injectable clock.** Anything time-dependent reads from an injected clock, not
  the system call. Otherwise it is untestable and flaky. `check: grep direct time calls`

## Frontend

- Never import an icon package that ships a barrel file. A 5000-icon barrel
  forces the dev server to pre-bundle everything and destroys iteration speed.
  Pick a tree-shakeable set and enforce it. `check: import denylist` `want: guard`
- **No type casts.** `as` is banned. If the compiler says the type is wrong, fix
  the type - a cast hides the bug rather than resolving it.
  `check: grep for as-casts` `want: guard`
- Named exports with inline prop types. No default exports, no `FC<Props>`
  annotations. Makes rename-across-repo mechanical. `check: lint rule` `want: guard`
- **Route guards are not React.** Auth lives in the query cache and is read from
  the router's `beforeLoad`. Doing auth navigation in an effect produces a logout
  race that is very hard to reproduce. `check: grep auth nav in effects`
- Every app has an error boundary at its root and around any form that mutates
  money. Without one, an unexpected response shape blanks the whole app.
  `check: grep error boundaries at app roots`
- **i18n and RTL from the first screen, not retrofitted.** A second locale with
  RTL constrains component library and layout primitives. Retrofitting is a
  rewrite; starting with it is a mild tax. `check: locale file parity`
- i18n keys must exist in **every** locale file. No inline fallback strings - a
  fallback is a missing translation that never gets reported.
  `gate: validate:i18n`

## Documentation integrity

- A documented command that does not work is worse than an undocumented one. If a
  build or test command breaks, the doc and the command are fixed in the same
  commit, or the doc is deleted. `want: gate` - extract fenced commands from
  orientation docs and dry-run them
- **No repo todo file.** Open work lives in one register. Scattered `todo.md` /
  `next.md` files become closure reports in disguise, and the closure narrative
  then has nowhere to go but the worklist. `check: no such files exist`
