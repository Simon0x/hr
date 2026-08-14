# Changelog

Notable changes per release. Versions follow [semver](https://semver.org).

## 1.0.0-rc.1

First public release candidate. `hr` is a single Go binary: a governance layer
that dispatches work to a coding-agent CLI and holds it to a policy, a tool
grant, and an append-only attested record it cannot rewrite.

- **A seat's declared tool grant is now the grant it actually runs under.**
  `ToolsForDepartment` truncated every entry at `(`, so a SKILL.md granting
  `Bash(hr impact *)` — one command — reached the agent as bare `Bash`, the
  whole shell; and `disallowed-tools:` was never parsed at all, so declared
  denials were decorative. Both are fixed, which means seats now run under
  materially narrower authority than before. `ToolsForDepartment` is replaced
  by `GrantForDepartment`, returning a `harness.Grant{Allow, Deny}`, and the
  `Harness` interface takes that grant in place of `tools []string`.
- **Signing keys have a validity window, a revocation, and a rotation
  procedure.** `trust/keys.json` entries may carry `notBefore`, `notAfter` and
  `revokedAt`; all optional, so an existing bundle is unaffected. They are
  checked against *when the artifact was signed* — read from the ledger's
  `emitted` entry, which is hash-chained and so cannot be backdated — never
  against now. That distinction is the point: revoking a key refuses
  everything it signed after the revocation instant while leaving the
  legitimate history behind it verifiable. An artifact with no `emitted` entry
  is refused rather than checked against the current time. Rotation and
  revocation procedures are written into `trust/keys.json` itself. Closes
  register finding SEC-001; issuance stays manual, and short-lived SVIDs from
  a SPIFFE issuer remain the better end state.
- **Every constraint names how it is enforced.** Five bound at `check:` with
  nothing behind them. Four already had enforcement and only needed the
  binding to say so — three tests written here (an unaffordable step never
  reaches policy or the model, budget refusal short-circuits before policy, a
  dry run decides and records while invoking nothing) and `validate:ledger`,
  whose schema already required a non-empty actor on every entry. The fifth
  needed a seam first: the per-goal arithmetic behind `hr budget report` moved
  out of `cmd/` into `budget.Report`, which reports `PerAccepted` guarded by
  `Denominated` — with nothing accepted there is no rate, and it says so
  rather than printing spend that reads like one. Coverage at gate or above
  moved from 70% to 87%, and no constraint carries `want:` any more. Closes
  register finding DUR-001.
- **A quarantined step can be resumed, and the watchdog can actually
  quarantine.** `QuarantineRepeatedFailures` marked the newest *failed* row
  quarantined, but the repopulate loop has already inserted a live one by
  then and `quarantined` counts as active - so it produced two active rows for
  one step key and died on the unique index, failing exactly when it was
  needed. It now quarantines the live row. Exceptions filed by `dispatch`
  carry `stepKey`, and both filers derive their subject digest from
  `dispatch.StepKey`, so the three disagreeing recipes that made an exception
  unmatchable to its job became one. Closes register finding DUR-003.
- **Spawned agents are confined by the kernel.** Every control above this
  needed the harness to cooperate: the tool grant holds because a CLI honours
  its matcher, the policy guard because the harness runs a hook. This one
  does not. `internal/sandbox` is the seam - a policy naming writable roots, a
  provider wrapping argv before spawn - with `Landlock` on Linux and a `None`
  that reports its own name so an unconfined deployment is stated rather than
  assumed. hr is its own helper: a ruleset applies to the calling process and
  Go has no hook between fork and exec, so the wrapper re-enters hr as
  `hr confine ... -- <argv>`, which restricts itself and execs. The agent
  starts confined and cannot un-confine, since a ruleset is irrevocable, and
  `hr confine` refuses to run at all rather than run unconfined. Writes only:
  reads and execution are unrestricted, because the denials a seat declares
  are `Edit`/`Write` and restricting reads breaks a toolchain long before it
  stops anything. `HR_SANDBOX=off` opts out. Verified with a real agent, which
  wrote inside its workspace and was refused `/etc` by the kernel. Closes
  register finding SEC-004.
- **The policy engine decides at each tool call.** It was consulted once per
  step and then had no say in anything the agent did, so a flat per-department
  tool grant was doing the real constraining — much weaker than a
  five-dimension engine. `internal/guard` scores a single call on what that
  call can do (read-only `R0`, file mutation `R1`, shell `R2`, unwalkable-back
  commands `R3`/irreversible, unclassified tools as mutating), and `hr guard`
  is the subcommand a harness runs per call. It fails closed: any error at all
  denies. `escalate` denies too, because there is nobody to escalate to inside
  a running invocation. The harness wires it through Claude Code's
  `PreToolUse` hook with a per-invocation settings file, and refusals are
  folded into the ledger by the dispatching process rather than appended by
  the guard, which would have several processes writing one chain with nothing
  ordering them. Verified end to end against the real CLI: with the grant
  explicitly allowing `Bash(rm *)`, a `rm -rf` was refused, the file survived,
  and the entry names the policy digest that refused it. Closes register
  finding SEC-003.
- **`HR_ROOT` overrides project-root detection.** The guard runs as a hook in
  the agent's working directory, which need not sit under the project, so
  walking upward from cwd found the wrong root or none.
- **A harness that cannot enforce a grant refuses the work.** The constraint
  was stated when scoped grants landed but nothing implemented it: there was
  no way for a harness to declare what it can hold to, and no check. `Harness`
  gains `CheckGrant`, and `Select` wraps every harness it returns, so the
  refusal happens at the seam rather than at each call site that must not
  forget it. Dispatch records a refusal as blocked - an infrastructure block,
  not evidence about the work, because nothing ran.
- **Durable state is behind one definition.** `internal/persistence` owns the
  ledger and artifact-store vocabulary; `File` and `Postgres` provide it, and
  the server depends on the interface instead of naming `pgstore` at each
  call site - the same leak `Harness` was created to close. Jobs, identities
  and exceptions stay Postgres-bound: they are server-only concepts with no
  file counterpart to be swapped for, so a seam there would be one role
  pretending to be three. One behaviour suite runs against both providers,
  which immediately caught them disagreeing about artifact identity - the
  file store derives the id from content on read while Postgres took the
  struct's field, so the same evidence could be filed under two names. Both
  now derive it through `store.DigestID`. Every consumer goes through the
  seam - `dispatch`, `emit`, `budget`, `workflow`, `exceptions`, the daemon
  and the CLI included - so no code names a backend but the two providers
  and the composition roots that pick one. The definition lives apart from
  its providers (`internal/persistence` vs `.../pgprovider`), because a
  Service Definition importing a backend drags every dependency into every
  consumer, and here it was a literal import cycle.
- **The server never trusts a client-supplied actor.** `POST /v1/jobs/claim`
  took the claimant from the request body, and the worker fed `HR_ACTOR` into
  every entry it sent, so ledger attribution for claimed work was
  self-asserted - a worker could write entries under any name. The server now
  stamps the authenticated identity on the claim and on every entry a
  completion writes, prior entries included, and `claimedBy` is gone from the
  wire rather than left as a field nothing reads. `HR_ACTOR` still names the
  actor for local `hr run`, which has no identity to resolve; against a
  server it is a display label.
- **An identity's empty department grant now means no departments, not all of
  them,** and `hr identity create` requires `--departments`. An omitted scope
  used to be a silent grant of the whole system, which is the wrong direction
  for a flag to fail in. Pass `--departments all` for an unscoped identity, or
  name them.
- **`hr daemon`.** A persistent process: watches `.hr/artifacts/` via
  filesystem events and re-derives `next` on every change instead of waiting
  to be re-invoked, dispatches through a bounded worker pool, and defaults to
  watch-and-report only - `--execute` is required to actually dispatch, the
  same opt-in posture `run` already had. Crash-safe: a `claimed` ledger entry
  is written before any agent invocation and matched against a terminal entry
  on the next startup, so an interrupted dispatch is surfaced for review
  rather than silently retried or dropped. `hr daemon --once` runs a single
  replay-and-drain pass for a cron-triggered deployment instead of a
  long-lived process. Backed by a `bbolt` read-model cache under
  `.hr/index.bbolt` - rebuilt from the ledger and artifacts on every start,
  never itself a source of truth.
- **Externally derived text is fenced before it reaches a model, and the
  ledger says when an artifact rests on it.** Discovery's leads read the open
  web (`WebSearch`, `WebFetch`) and their raw output was interpolated into the
  synthesis prompt behind `--- lead "name" ---` delimiters the content itself
  could forge, under an instruction to use those findings - and the result is
  a signed, contract-validated artifact. Lead output is now wrapped in a
  per-prompt random fence that content cannot close (an occurrence of the
  fence id inside the content is neutralised, not dropped), introduced by a
  preamble stating that fenced material is reported data and never
  instruction. The synthesis entry carries `request.untrustedInput`, so an
  evidence chain built partly from outside material can be told apart from
  first-hand work.
- **Every ledger entry that invoked a model now records what it was asked.** A
  new `request` object carries the prompt by digest, the tool grant, the
  harness name, and the procedure path with a digest of its content. The
  ledger could already prove that a seat ran, under which policy and at what
  cost, but not what it was told to do or what it was allowed to touch - and
  editing a `departments/*.md` procedure silently changed what past entries
  meant. Recorded on the `claimed` entry *before* invocation on the direct
  path, so an interrupted run still says what was asked. `Harness` gained a
  `Name() string` method so the entry can name the agent that produced the
  work. Existing chains are unaffected: the field is omitted when absent, so
  entries written before this re-canonicalize to identical bytes.
- **The worker's fan-out now records its lead invocations.** It previously
  reported only job-level success or failure, so N model invocations produced
  no individual record on that path while `hr run` recorded one each. Job
  completion carries them as `prior` entries, appended ahead of the terminal
  entry in the same transaction, so either the whole job's record lands or
  none of it does. `prior` is deliberately not a general ledger-write API:
  the server accepts only `action` entries that carry a `request` and no
  artifacts, bounded per job, so a valid lease cannot be used to forge a
  decision. Both paths build their entries from one `dispatch.LeadEntries`,
  so they cannot drift into logging different sets.
- **`identities.departments` is enforced.** It was stored and returned but
  never checked, so any authenticated identity could act on any department.
  Both doors are now closed: `POST /v1/departments/{name}/run` returns 403 for
  an ungranted department, and `POST /v1/jobs/claim` narrows a worker's
  requested departments to its grant - returning 403 only when none overlap,
  so one worker binary can still serve identities of different scope without
  ever being handed ungranted work. `GET /v1/departments` lists only what the
  caller may run, so the UI never offers a button that would 403.
- **`hr next` derives real policy facts instead of a static per-department
  table.** `risk` and `reversibility` are now read from the actual `GOAL` and
  `CHANGE` artifacts a step concerns, falling back to the old per-department
  defaults only when nothing more specific exists.
- **A `VERDICT` with unmet or unverifiable criteria now routes back
  automatically** - to Engineering or Product respectively - instead of
  leaving the chain dead-ended in `next`'s `blocked` list forever.
- **An `authority` escalation now files a real `EXCEPTION` artifact** instead
  of only a ledger line, using the `class` taxonomy AGENTS.md already
  described (`irreducible-judgment`, `missing-policy`, ...).
- Signing now uses `github.com/secure-systems-lab/go-securesystemslib/dsse`
  for PAE encoding rather than a second hand-rolled implementation - verified
  to produce byte-identical signatures to the retired TypeScript version
  against the same key and payload. Contract validation now uses
  `github.com/santhosh-tekuri/jsonschema/v6`, a spec-compliant 2020-12
  validator, in place of the hand-rolled keyword subset - see
  [`contracts/README.md`](contracts/README.md).
- Nothing behavioural changed in the ported subcommands beyond what's listed
  above as Added - every one was verified against its retired Bun
  counterpart on real repo data before being trusted, and several were
  byte-identical (`ledger show --json`, `impact`, signed payload bytes).
