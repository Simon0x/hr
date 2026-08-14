# Acme Ledger

AGENTS.md is the index - short rules inline, deep-dives in `docs/`. Read the linked doc whenever the area applies.

Multi-tenant billing service. Go + Postgres, deployed as containers on a single
host. Money is integer cents throughout.

## General

- **Port:** 8080. `make dev` runs the stack; the user runs it, not you.
- **No repo todo file.** Open work lives in `docs/audit/REGISTER.md`.

## Never run these

- **NEVER run `make migrate` or `make deploy`.** Wait for the user.
  (`make migrate-diff` is allowed - it only writes local files.)
- **`make generate-client` is user-only.** It needs the API running on :8080.

## Money

- Amounts are `Cents` (int64). Never float. `compile`
- `Cents` is a distinct type, so `a + b` on raw ints does not typecheck. Use
  `money.Add` / `money.Sub`. `compile`
- Display formatting happens once, at the edge, in `internal/fmtmoney`. Never
  inline. `guard: validate:money-format`

## Tenancy

- Every query against a tenant-scoped table runs inside `WithTenant(ctx, id)`.
  The pool type rejects a bare connection, so the unsafe call does not build. `compile`
- A row-level policy refuses reads without `app.tenant_id` set. Backstops the
  escape hatch. `db`
- `Unscoped()` is the deliberate, greppable escape hatch. Every use needs a
  one-line reason. `guard: validate:tenant-scope`

## Schema

- Source of truth is `db/schema.sql`. Never hand-write a migration; edit the
  schema and generate the diff. `check: review every generated migration`
- Exception: column renames. The differ emits DROP + ADD, which loses data.
  Hand-write the `ALTER TABLE ... RENAME COLUMN`. `check: review every generated migration`

## HTTP

- Every success returns `{data, meta}` via `httpx.Envelope`. Downloads and 204s
  opt out explicitly. `test: envelope_test.go`
- Errors return a coded type from `errs/`. Never a bare string. `guard: validate:error-codes`

## Coverage

`compile` 3 · `db` 1 · `guard` 3 · `codegen` 0 · `test` 1 · `gate` 0 · `check` 2 · `none` 0 - **at gate or above: 80%**

Recomputed at each audit close. If nothing climbed a rung, the audit bought nothing durable.

## Deep-dive docs

Read the matching doc whenever the area applies - these capture rationale + edge cases that don't fit inline.

- **`docs/tenancy.md`** - read when touching `WithTenant`, `Unscoped`, any
  `*_tenant.sql` query, or adding a tenant-scoped table. Covers: the pool split,
  why the policy exists as well as the type, and the two legitimate `Unscoped` uses.
- **`docs/money.md`** - read when touching `Cents`, rounding, proration, or
  invoice totals. Covers: rounding direction per operation, the proration
  algorithm, and why totals are recomputed rather than summed.
- **`docs/audit/REGISTER.md`** - read at task start when the task touches a
  subsystem with open findings.
- **`docs/audit/records/<date>-<slug>/`** - frozen records. Never truth about the present.
