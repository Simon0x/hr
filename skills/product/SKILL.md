---
description: Turn a surviving hypothesis or an open finding into a spec whose every acceptance criterion is executable. Use when something has been decided worth building but nobody has written down what done means.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Grep, Glob, Bash(${CLAUDE_PROJECT_DIR}/hr emit*), Bash(hr emit*), Bash(./hr emit*)
---

Specify: `$ARGUMENTS` (a hypothesis id, a finding id, or a claim).

Read `${CLAUDE_PLUGIN_ROOT}/departments/product.md` and follow it.

In short: a criterion that cannot be executed is not a criterion. Each
acceptance line carries the command, request or assertion that settles it, and
the test is mechanical - could someone who has never met you run this and get
the same answer?

Name **which link** of the hypothesis this spec buys evidence for. Without it,
"we shipped and usage was low" cannot distinguish a wrong mechanism from a bad
implementation, and those have opposite next moves.

Fill `out-of-scope[]`. An empty one usually means under-thought rather than
trivial.

Emit a `SPEC` as an in-toto Statement via `hr emit`; the shape is in
`${CLAUDE_PLUGIN_ROOT}/contracts/predicates/spec.schema.json`. A criterion that
resists execution is a **finding** - record it and name who decides, rather than
phrasing it to look automatic.
