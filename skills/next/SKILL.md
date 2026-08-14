---
description: Say which department runs next and why, derived from the artifacts that exist. Use when you do not know what to do next, or want to see what a gate is holding.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr next *), Bash(hr next*), Bash(./hr next*)
---

If `${CLAUDE_PROJECT_DIR}/hr` does not exist yet, build it first: `cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr`.
Then run `hr next` from `${CLAUDE_PROJECT_DIR}` and report what it says.

Position is **derived** from the artifact store, not tracked beside it - the
spine is already a state machine, so there is no workflow state to drift from.

Two lists come back and they mean different things. **ready** is work whose
inputs exist. **blocked** is a gate holding, not a queue backing up: a
hypothesis below `behaviour`, a criterion unmet, a rollback never rehearsed.
Report which gate and what would clear it, never how to get around it.

If the store is empty the answer is a `GOAL` - see
`${CLAUDE_PLUGIN_ROOT}/contracts/predicates/goal.schema.json`.
