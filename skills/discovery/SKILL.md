---
description: Test whether an idea holds by breaking its mechanism into links and finding which one fails. Use when considering building something, evaluating a market, or asking whether a hypothesis is worth acting on.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Grep, Glob, WebSearch, WebFetch, Bash(${CLAUDE_PROJECT_DIR}/hr emit*), Bash(hr emit*), Bash(./hr emit*)
context: fork
---

Test this claim: `$ARGUMENTS`

Read `${CLAUDE_PLUGIN_ROOT}/departments/discovery.md` and follow it.

In short: restate the claim as a chain of *because*, then work it as a frontier
of leads rather than a checklist. Take the cheapest open lead first. Each lead
either settles a link, spawns better leads, or both. Follow independent leads in
parallel; a dependent lead waits for its parent.

**A break is usually not a stop.** Only a terminal break ends the search: the
space is dead, nobody can buy, it is illegal everywhere. A *mechanism* break
means this why is wrong while the pain may be real, and a *scope* break means
right idea, wrong buyer. Both mean re-chain from what you just learned and keep
spending the budget. Stopping at a mechanism break is analytically correct and
strategically useless.

Otherwise stop when the frontier empties or the budget is spent. Never stop
quietly: an unfollowed lead is a finding.

You did not write this chain, which is why you can see its weak link. Do not
defend it, and do not accept a repositioned claim as a rebuttal. That is a new
chain and it needs testing like any other.

**Write a document, not a trace.** Follow the reporting format in the procedure:
a title restating the claim, a two-sentence recommendation that stands alone,
then where this points, why the original fails, what we now know as a table, what
was not checked, and sources.

Sentence-case headings. Rungs in a table column, never inline in prose. One fact
per row. Never report the chain itself; it is working notes.

**Every source is a clickable markdown link to the page that actually says it**,
not a bare name and not the site's front door. Where no URL exists, say what the
source was and why it cannot be linked.

Give every finding an evidence rung. Say plainly when one rests on `stated` or
below, which is the rung that feels like research and is not.
