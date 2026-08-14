# Scaffold

Copy into a new repo, **delete what does not apply, keep the reasoning**.

These are worked examples for a fictional service, not blank templates. Editing a
working example down is reliable; filling in `<placeholders>` is not - blanks get
copied verbatim and become the first thing that rots.

| File | Goes to | Then |
|---|---|---|
| `AGENTS.md` | repo root | Replace the constraints with yours. Keep the section order, the index line, and the coverage line. |
| `REGISTER.md` | `docs/audit/REGISTER.md` | Empty the findings, keep the conventions. |
| `settings.json` | `.claude/settings.json` | Merge with what is there. The `deny` rule on `docs/audit/records/**` is the point - and it stops one segment short of the register on purpose. |

**There is nothing to copy for the commands.** The `/hr:*` entry points ship
inside the plugin and arrive with `/plugin install hr@hr`; the shims in `skills/`
and `agents/` resolve their own procedures through `${CLAUDE_PLUGIN_ROOT}` and
need no counterpart in the consuming repo. Only the three files above cross over,
because only they hold project-specific content.

A shim carries a tool grant and a pointer at a procedure, and no logic of its own
- see [`../COMMANDS.md`](../COMMANDS.md). Delete every shim and the departments
still work: a human runs the script and follows the procedure.

`CLAUDE.md` has no file here because it is three lines:

```markdown
# <Project>

See [AGENTS.md](./AGENTS.md) for project constraints and conventions.
```

Constraints live once, in `AGENTS.md`, so every agent reads the same source.
Duplicating them into `CLAUDE.md` gives you two truths read by different tools -
so neither reader ever sees the contradiction.

## What to keep from the example

The parts that are non-obvious rather than project-specific:

- **The index line**, verbatim. It tells an agent the file is complete for its
  purpose, so a gap means follow a link, not go spelunking.
- **"Never run these"** as its own section, near the top. Naming the *safe*
  variant alongside the banned one is what makes the ban credible.
- **The coverage line.** It is the health metric, and writing it by hand until
  you build the tool is fine.
- **Read-triggers on doc links** - name symbols and files, not topics. "Read when
  working on billing" is unusable; "read when touching `Cents`" fires reliably.
