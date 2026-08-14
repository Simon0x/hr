---
description: Audit a codebase against its own declared constraints, find where documentation has stopped being true, and promote what is learned back into enforcement. Use when a structural change has landed, when the register looks stale, or when asked to review the health of a project.
---

Audit: `$ARGUMENTS` (a scope, or empty for the whole project).

Read `${CLAUDE_PLUGIN_ROOT}/method/audit-loop.md` and follow it. The enforcement
vocabulary it uses is in `${CLAUDE_PLUGIN_ROOT}/method/enforcement-ladder.md`.

Start by validating the knowledge base before trusting it: resolve every
`Evidence:` path in `${CLAUDE_PROJECT_DIR}/docs/audit/REGISTER.md`, and run
everything bound at `gate:` or above. If most of the register comes back stale,
stop - the finding is that the knowledge base decayed, and re-grounding it is the
work.

Do not hand-check anything a gate already covers. If you catch yourself doing so,
that is a missing gate, and it belongs in the register as one.

Close by asking of every finding: **can this go up a rung?** An audit that
promotes nothing bought nothing durable.
