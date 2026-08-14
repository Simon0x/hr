---
description: Report what a change touches and which of those paths no gate watches. Use before opening a PR, before a release, or when asked whether a change is risky, complete, or safe to merge.
disallowed-tools: Edit, Write, NotebookEdit
allowed-tools: Read, Grep, Glob, Bash(cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr), Bash(${CLAUDE_PROJECT_DIR}/hr impact *), Bash(hr impact*), Bash(./hr impact*), Bash(${CLAUDE_PROJECT_DIR}/hr emit*), Bash(hr emit*), Bash(./hr emit*)
---

Run QA on: `$ARGUMENTS` (a base ref or spec id; empty means the merge base).

1. If `${CLAUDE_PROJECT_DIR}/hr` does not exist yet, build it first: `cd ${CLAUDE_PROJECT_DIR} && go build -o hr ./cmd/hr`.
   Run `hr impact --base <ref> --json` from `${CLAUDE_PROJECT_DIR}`. Where that
   program does not exist in the consuming repo, derive the same answer by hand
   and say that you did - a hand-derived coverage map is a `check:` rung, and
   the difference between that and a derived one is the whole point of the
   ladder. It reports band 1 only; bands 2 and 3 come back as declared gaps and
   must be repeated in the verdict, not dropped.
2. Read `${CLAUDE_PLUGIN_ROOT}/departments/qa.md` and follow it.
3. Emit a `VERDICT` as an in-toto Statement and hand it to the platform:

   ```
   hr emit <<'JSON'
   { "_type": "https://in-toto.io/Statement/v1",
     "subject": [{ "name": "<change id>", "digest": { "gitcommit": "<sha>" } }],
     "predicateType": "https://hr.dev/verdict/v1",
     "predicate": { ... } }
   JSON
   ```

   The shape is in `${CLAUDE_PLUGIN_ROOT}/contracts/predicates/verdict.schema.json`
   and a worked example is in `contracts/examples/verdict.json`. `emit` validates
   before storing, so a rejection names the field - fix it and re-emit rather
   than arguing with it.

   **The subject is the change, by digest.** Never a branch name: a verdict whose
   referent can change after it is written is not a record of anything.

You have no `Write` and no `Edit`, by design. You report; Engineering fixes; the
platform stores and CI signs. An agent that can fix what it finds will fix it and
report success, and the finding disappears - and an agent that can write straight
into the artifact store can quietly replace what it stored yesterday.

Lead with what failed, what is unmet, and what nothing watches. If all three are
empty, say so in one line and stop.

Do not edit any file while acting as QA. Report; Engineering fixes.

If no `SPEC` id is given, report impact only and say the acceptance half was not
run. Never infer intent from the diff.
