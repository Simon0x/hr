# Agent Integration

The four layers are not just a filing system for humans - they map onto an
agent's context budget, and that mapping is what makes them work.

An agent has a fixed attention budget and no way to tell a current document from
a frozen one. Both problems are structural, and both are solved by loading each
layer differently.

---

## State the file's job in its first line

Every `AGENTS.md` opens with the same sentence:

> AGENTS.md is the index - short rules inline, deep-dives in `docs/`. Read the
> linked doc whenever the area applies.

That one line does real work. It tells the agent that the file it just
auto-loaded is *complete for its purpose* and that anything absent is
deliberately elsewhere - so the correct move on a gap is to follow a link, not to
go spelunking through `docs/` or infer a convention from nearby code.

Use it verbatim, unchanged, in every repo.

## Index entries carry read-triggers, not just links

A bare list of filenames makes the agent guess. Each O index entry states **when** to
read it and **what** it covers, so the decision is made without opening the file:

```markdown
- **`docs/forms.md`** - read when touching `useForm`, `getInputProps`, a
  `<Stepper>`, or any dynamic list. Covers: resolver choice, controlled-vs-
  uncontrolled mode, edit-page hydration, the `min()`-on-optional-fields footgun.
```

The trigger names symbols and files, not topics. "Read when working on forms" is
unusable; "read when touching `getInputProps`" fires reliably, because the agent
is looking at that symbol when the question arises.

## The pointer pattern

`AGENTS.md` holds the substance. `CLAUDE.md` points at it.

```markdown
# <Project>

See [AGENTS.md](./AGENTS.md) for project constraints and conventions.
```

That is the entire file.

**Why this direction:** `AGENTS.md` is the convention most coding agents now read
- Codex, Cursor, Copilot, Gemini CLI. `CLAUDE.md` is what Claude Code loads
natively. Writing the constraints once in `AGENTS.md` and pointing at it from
`CLAUDE.md` means one source of truth serving every agent.

Writing them *twice* creates exactly the duplicate-truth failure the knowledge
model forbids - and it is a worse instance than most, because the two copies are
read by different tools, so neither reader ever sees the contradiction.

**Do not symlink.** A pointer file survives Windows checkouts and lets
`CLAUDE.md` carry harness-specific notes (permissions, skills) that would be
noise to other agents.

## Context budget by layer

| Layer | Loading | Budget |
|---|---|---|
| **C CONSTRAINTS** | Auto-loaded, every session | **≤100 lines.** Hard. |
| **O ORIENTATION** | Read on demand, indexed from C | Unbounded, one subject per file |
| **R REGISTER** | Read at task start when the task touches an open finding | Shrinks as findings close |
| **H RECORD** | **Never loaded.** Actively excluded. | Unbounded, inert |

### The C budget is self-enforcing

`AGENTS.md` is in context for every task, so every line competes with the actual
work. A hundred lines is roughly the point where an agent starts skimming instead
of obeying.

When the file outgrows the budget, the fix is **not** a second file. It is to
find the constraints that can climb a rung and build the enforcement - anything
at `gate:` or above states in one line, because the script carries the detail. A
`check:` needs its whole procedure inline.

So constraint bloat pushes toward higher rungs, which is the direction you
already want. Splitting the file instead relieves the pressure and stalls the
climb.

### H exclusion is load-bearing

This is the one that bites hardest in practice.

A mature project accumulates dozens of archived analyses. They are long, they are
topical, and they discuss the domain in far more depth than any current doc. An
agent grepping a domain term gets the eleventh revision of a delta analysis
ranked alongside `docs/ARCHITECTURE.md` - and the archived one is both more
verbose and more specific, so it often wins.

The agent then reasons confidently about the system as it existed at some earlier
version, with no signal that anything has changed.

Two mechanisms, both cheap:

1. **Path convention.** Everything frozen lives under
   `docs/audit/records/<date>-<slug>/`. The date in the path is the staleness
   signal, visible in every search result.
2. **Deny the read.** In `.claude/settings.json`:

```json
{
  "permissions": {
    "deny": ["Read(./docs/audit/records/**)"]
  }
}
```

Records stay in git, greppable by humans, and out of agent context by default.
When an audit genuinely needs to mine an old record, that is a deliberate
override - which is the correct friction, because reading a frozen record is
always a deliberate act.

### The deny rule must not swallow the register

The rule targets `docs/audit/records/**`, not `docs/audit/**`, and the extra path
segment is the entire point. The R register lives at `docs/audit/REGISTER.md` -
inside the same directory, and the one file an agent is *required* to read at
task start. A rule written against `docs/audit/**` denies both, so the register
becomes unreadable to every agent that is supposed to consult it, silently and
with no error that names the cause.

Frozen history and the live worklist share a parent directory for human
convenience. They must not share a permission rule.

### What the deny rule does not do

It withholds the `Read` tool. It does not withhold the file.

An agent holding `Bash` can still reach a denied path through `cat`, `grep`,
`sed`, a test runner, or anything else that opens a file - the permission layer
matches tool invocations, not the bytes underneath. Treat the rule as **an
attention budget, not a security boundary**: it exists so frozen records stop
crowding out current truth in a search result, and it is honest about nothing
more.

Where the boundary needs to hold against a determined process rather than a
default, it needs OS-level enforcement - a sandbox, a mount, or file permissions.
Say which one you are relying on; do not let a `deny` entry stand in for a
control it cannot provide.

## Session protocol

What a well-behaved agent does, in order:

1. **Read C.** Auto-loaded. These are the rules; they are not advisory.
2. **Check R** if the task touches a subsystem with open findings. Working on a
   known-broken area without reading the register wastes a cycle rediscovering it.
3. **Read O on demand**, following the index in C. Never pre-load `docs/`.
4. **Never consult H.** If a record seems necessary, the fact you want has
   probably not been promoted - promote it, then use it from C or O.

State it in the file itself: *"Read on demand - don't pre-load."* Keep that
line in every project.

## Delegating to subagents

A subagent inherits none of the caller's judgment, so it gets **C plus the one
O document its task needs** - never the whole `docs/` directory. Pointing a
subagent at a doc directory reliably produces a summary of the directory rather
than the task.

State the constraint bindings relevant to its task explicitly in the prompt. A
subagent that does not know a `guard:` exists will re-derive the check by hand,
which is exactly the cost the guard was built to remove.

## Commands the agent must not run

The most important thing the C layer can say is what *not* to execute:

```markdown
- **NEVER run `task db:migrate` or `task dev`.** Wait for the user.
  (`task db:diff` is allowed - it only writes local files.)
- **Gen:** `bun run api:generate` - **user only**. NEVER run this yourself.
```

Two properties make these hold:

- **The safe variant is named.** A rule banning a whole command family gets
  ignored the first time someone legitimately needs a migration generated.
  Permitting `db:diff` explicitly is what makes prohibiting `db:migrate` credible.
- **The reason is one clause, not a paragraph.** "It only writes local files"
  and "it needs the API running on a known port" are enough to generalize from.

Back the important ones with `permissions.deny` in `.claude/settings.json` so the
prohibition survives an agent that skims. Prose is `check:`; a deny rule is
closer to `guard:`.

## Working with non-Claude agents

Nothing in this framework is Claude-specific:

- **Constraints** are `AGENTS.md`, the cross-agent convention.
- **Gates** are ordinary package scripts (`bun run validate:*`). Any agent - or
  any human, or CI - can run them and read the exit code.
- **The register** is plain markdown with a fixed schema.
- **Records** are dated directories.

The only Claude-specific artifacts are the `CLAUDE.md` pointer and the
`permissions.deny` rule. Both are additive; an agent that ignores them still gets
correct behavior from `AGENTS.md`, just without the H protection.

That portability is deliberate. The framework outlives any particular tool, and
the knowledge is the asset - not the harness that happens to read it.

## Automating the loop

Two hook-shaped opportunities, noted but not built:

- **On stop** - recompute constraint coverage and flag any `Evidence:` path in
  the register that no longer resolves. Both are cheap, and both catch the exact
  decay that turns a concerns document into fiction.
- **On edit of a gated path** - run that path's `gate:` script rather than the
  whole suite.

These belong to the automation workstream, not this one.
