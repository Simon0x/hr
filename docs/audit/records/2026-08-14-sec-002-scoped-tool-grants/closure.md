# SEC-002 closure: a seat's Bash grant is scoped to named commands

**Closed:** 2026-08-14

## What the finding claimed

The judgment seats withheld `Edit` and `Write` but held `Bash`, so a seat
could write a file through a shell and the denial expressed intent rather
than enforcing it. `departments/README.md` called that grant `compile`-rung
separation of duties when it was `check:` at best.

## What this pass found

The claim was true, and worse than recorded. The SKILL.md files did not grant
bare `Bash` - they already declared scoped commands:

```
allowed-tools: Read, Grep, Glob, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr impact *)
disallowed-tools: Edit, Write, NotebookEdit
```

The dispatcher discarded both. `ToolsForDepartment` truncated every entry at
`(`, turning a grant to two commands into bare `Bash`, and it never read
`disallowed-tools:` at all. So hr widened its own declared policy on every
dispatch and dropped every denial. The register recorded the symptom; the
cause was one line of parsing.

## What this pass did

- `GrantForDepartment` replaces `ToolsForDepartment` and returns
  `harness.Grant{Allow, Deny}`: entries keep their argument patterns, the
  deny list is read, and `${CLAUDE_PROJECT_DIR}` resolves against root.
- The `Harness` seam takes that grant. `claude.go` passes each entry as its
  own argv value across `--allowedTools` / `--disallowedTools`, because an
  argument pattern contains spaces and would be split by a comma join.
- The declarations were completed to match what each procedure instructs -
  `hr emit`, `hr impact`, `hr next`, `hr recall`. Those ran under the bare
  `Bash` the bug produced; with the bug fixed they had to be granted or every
  seat would break.
- `Select` wraps every harness so an invocation whose grant the harness
  cannot enforce is refused rather than run wider than asked
  (`harness.ErrGrantUnenforceable`), and dispatch records that as blocked -
  an infrastructure block, not evidence about the work.
- The ledger records both halves of the grant (`request.tools`,
  `request.deniedTools`), so an audit cannot read a grant too wide.

## Evidence

- `internal/dispatch/tools_test.go` - a `Bash(...)` entry never collapses to
  bare `Bash`; the deny list is parsed; an unconfigured seat stays read-only;
  and the **real shipped skills** parse to scoped grants, so this cannot
  silently regress.
- `internal/harness/grant_test.go` - each entry reaches the CLI as its own
  argument.
- `internal/harness/guard_test.go` - a harness that cannot enforce a grant
  never runs.
- `internal/dispatch/dispatch_test.go` - end to end, the harness receives the
  seat's scoped grant and denials, and the ledger records them.

## What is left, and is not this finding

Enforcement is the harness's. hr passes the right constraints and refuses a
harness that says it cannot hold them, but there is no OS-level confinement
behind the paths that matter - the second half of SEC-002's stated fix. That
residual is carried as its own finding rather than left inside a closed one,
because it is a different control at a different rung.
