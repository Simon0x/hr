---
description: Work out what change to actually make, at the lowest rung that answers the question, and what it will touch. Use before starting engineering work, when a problem is known but the fix is not, or when asked whether a change is bigger than it needs to be.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Grep, Glob, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr impact *), Bash(hr impact*), Bash(./hr impact*)
---

Intake for: `$ARGUMENTS` (a change, a problem, a spec id, or a finding id).

Read `${CLAUDE_PLUGIN_ROOT}/departments/engineering.md` and follow it. The
observability rungs it references are in
`${CLAUDE_PLUGIN_ROOT}/departments/ops.md`.

Two modes. If a change is proposed, interrogate it downward - smaller, lower
rung, later commitment. If only a problem is named, produce candidates at
different rungs and state the trade, because deciding what to build is the half
of this that pays.

Work the questions as a frontier, cheapest first, not as a questionnaire. Start
with the two that kill work for one question's cost: **which queue is this from -
a `SPEC` or an open finding?** and **is it already in the register?** Then the
rung, because the rung selects everything after it.

If `${CLAUDE_PROJECT_DIR}/hr` does not exist yet, build it first: `cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr`.
Run `hr impact --base <ref> --json` from `${CLAUDE_PROJECT_DIR}` for the constraints
that bind, the gates that cover, and what nothing watches. Where that program
does not exist yet, derive it by hand and say that you did - a hand-derived
answer is a `check:` rung, and the difference is worth stating.

**Ask what the revertible version is.** Changes get proposed at `migrate` when a
`flag` version settles most of the question in a day, and nobody asks this of
their own work.

Derive the rung from the diff wherever you can and declare it only where you
cannot. An answer is not evidence: "is this reversible?" returns yes for the same
reason "would you use this?" does. A declared rung the derivation contradicts is
a finding, not an override.

Report blast radius as a range - direct, transitive, and runtime reach that no
tool closes - with two coverage columns, gate and signal. A path with gates and
no signals ships confidently and fails silently.

**Write a brief, not a transcript.** Follow the format in the procedure: the
change restated in one line, a two-sentence recommendation that stands alone,
what this touches, what it costs to be wrong, failure modes with their
observability rung, and what was not decided. Never report the questions.

Do not edit any file during intake. You have no `Edit` tool by design - an agent
that can start the work will rush the intake to reach it. Hand off the brief.
