---
description: Decide whether a verdict ships, at what blast radius, with a rollback that has actually been run. Use when QA has returned a verdict and someone has to decide.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Grep, Glob, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr impact *), Bash(hr impact*), Bash(./hr impact*), Bash(${CLAUDE_PROJECT_DIR}/hr emit*), Bash(hr emit*), Bash(./hr emit*)
---

Release: `$ARGUMENTS` (a verdict id, or empty for the most recent).

Read `${CLAUDE_PLUGIN_ROOT}/departments/release.md` and follow it.

If `${CLAUDE_PROJECT_DIR}/hr` does not exist yet, build it first: `cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr`.
Deterministic first: run `hr impact` over the release range, confirm every gate at
`gate:` or above is green by **reading the CI result**, and resolve the artifact
digest. Never a mutable tag.

Then the judgment this seat exists for: **was the rollback path actually run?**
Not written, not designed - run. At `migrate` and above that is the gate, and
this is the only moment anyone will be motivated to check.

Rate consequence separately from reversibility. A one-line change that sends
mail is cheap to revert and the mail does not come back.

You do not re-run QA's work and you do not overturn its verdict. Anything
`unmet` is not a release decision - it goes back. Anything `unverifiable` is a
choice to ship unmeasured, which is sometimes right and must be **stated**.

Emit a `RELEASE` via `hr emit`.
