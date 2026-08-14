# Command Surface

One binary, `hr`. Every department has one Claude Code entry point; the binary
holds the deterministic scripts and the runtime underneath them.

## Running it

`hr` — the only command a normal run needs. Starts Postgres, then the server,
worker, and watchdog as goroutines in that one process, and opens the browser.
Already running? Same command attaches instead. Ctrl+C stops what it started.

`hr worker` / `hr watchdog` are standalone entry points for a teammate's
machine attaching to someone else's already-running `hr` — not needed for your
own.

## Departments

| Entry | Department | Runs | Judges | Emits |
|---|---|---|---|---|
| `/hr:discovery` | Discovery | link checks in cost order, fanned out per lead | which link of the chain breaks | `PROBLEM` / `HYPOTHESIS` |
| `/hr:product` | Product | - | intent → executable criteria | `SPEC` |
| `/hr:engineering` | Engineering (intake) | `impact` | what to build, at which rung | brief → `DESIGN` |
| `/hr:qa` | QA | `impact` | acceptance vs spec, risks | `VERDICT` |
| `/hr:release` | Release | `impact`, gates, digest | rollback viability, consequence | `RELEASE` |
| `/hr:audit` | cross-cutting | everything at `gate:`+ | `check:` rungs, O vs code | findings → register |
| `/hr:next` | - | `next` | nothing - it derives | which seat runs, and what a gate holds |
| `/hr:recall` | - | `recall` | nothing - it refuses rather than guesses | what is already known, with sources |

`/hr:next` is not a department - it reads the artifact store and says which
seat runs next, so a human isn't the router. Department definitions -
procedure, risk, reversibility, fan-out config - live in `capabilities/*.json`,
not in Go. Edit a department there; nothing recompiles.

### Not built

| Department | Would run | Would judge | Emits | Procedure |
|---|---|---|---|---|
| Ops | log and metric sweep | new versus known, digest attribution | `SIGNAL` | [`departments/ops.md`](departments/ops.md) |

Ops has its procedure and no shim - a human can run it today. It carries no
`/hr:` name until an entry point exists: a documented command that doesn't
work is worse than an undocumented one.

**Install**

```
/plugin marketplace add Simon0x/hr
/plugin install hr@hr
```

Update with `/plugin marketplace update`. Public repo, no credentials needed.

**Develop** — `claude --plugin-dir <path>` loads a local copy for that session.
`/reload-plugins` picks up `agents/` changes; `SKILL.md` edits apply immediately.

## Three layers, one truth

```
/hr:qa                shim       tool-specific, no logic
 └─ departments/qa.md    procedure  agnostic markdown - the truth
     └─ impact              program    exit code + JSON
```

A shim holds no rule the procedure doesn't, and asks no question the procedure
doesn't ask. Delete every shim and the department still works by hand - that's
the test for whether the layering is right.

## The `hr` binary

One Go program, `cmd/hr` — `go build -o hr ./cmd/hr`. Every subcommand below is
a function call within it, not a subprocess hop.

| Namespace | Holds |
|---|---|
| `validate:*` | one constraint each, `gate:`/`guard:` |
| `impact` | diff → what binds it, what covers it, what watches it |
| `run` / `daemon` | dispatch: budget → authority → invoke, cheapest question first |
| `worker` / `watchdog` | remote-team entry points; bare `hr` already runs both locally |

Ten gates run in CI and by hand — a gate a human can't run is a gate nobody
debugs. `AGENTS.md` carries the list and the coverage number.

`impact` (band 1) intersects a diff with the `@covers` globs gates declare in
their own headers, so there's no second map to maintain. Uncovered paths are a
**finding, not a blocker** — reported, never failing CI.

Every subcommand exits non-zero on failure and takes `--json` — the same
binary serves CI, a shell, and an agent without adapters.

## Shims

`skills/` and `agents/` hold the Claude Code shims — frontmatter plus a
pointer at a `departments/` procedure, no logic of their own. The tool grant
is the one thing that legitimately lives there: QA withholding `Edit` is
enforcement, not preference. See `departments/README.md`.
