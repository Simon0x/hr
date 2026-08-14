---
description: Ask what the system already knows about something, and get an honest answer including "nothing". Use before re-deriving anything, or when a decision smells like one that has been made before.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr recall *), Bash(hr recall*), Bash(./hr recall*)
---

Recall: `$ARGUMENTS`

If `${CLAUDE_PROJECT_DIR}/hr` does not exist yet, build it first: `cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr`.
Then run `hr recall "$ARGUMENTS"` from `${CLAUDE_PROJECT_DIR}` and report exactly
what comes back.

**Report "nothing stored" as an answer, never as a prompt to improvise.** If
recall says the store has no vocabulary for the question, that is the finding.
Do not fill the gap from your own knowledge and present it as something the
system remembers - afterwards nobody can tell the two apart, which is the one
failure this store exists to prevent.

Quote each fact with its source and confidence. `unverified` and `quarantined`
entries are reported as such and never restated as established.

Withheld entries are shown with their reason. A stale memory may still be true;
say it is overdue rather than dropping it silently.

See `${CLAUDE_PLUGIN_ROOT}/method/memory.md`.
