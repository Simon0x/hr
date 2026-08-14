# hr web UI

The dashboard for `hr-server`: a department chat view for manually dispatching
work, and an exceptions queue for resolving what a department escalated to a
human. React + TypeScript + Vite, styled with [Mantine](https://mantine.dev).

Talks to `hr-server` entirely over its REST/SSE API (`src/api.ts`) — see
`internal/hrserver` in the repo root for the server side, and
`internal/hrserver/auth.go` for the bearer-token auth every `/v1/` request
needs (get one with `hr identity create --name <you>`).

## Layout

- `src/App.tsx` — the whole UI: the department chat panel, the exceptions
  panel, the sign-in token gate, and the shared presentational bits
  (`PredicateFields`, status badges) that render an artifact's predicate
  generically.
- `src/api.ts` — the only thing that talks to `hr-server`: typed fetch
  wrappers, token storage (`localStorage`), and the two `EventSource`
  subscriptions that drive live updates.

## Running it

Normally you don't run this directly — the bare `hr` command (see the repo
root's `cmd/hr`) builds and embeds this into the `hr-server` binary via
`embed.go`. For frontend-only iteration against an already-running
`hr-server`:

```
npm install
npm run dev
```

Point it at a non-default server with `VITE_HR_SERVER_URL` (see `src/api.ts`,
defaults to `http://localhost:7777`).

## Scripts

- `npm run dev` — Vite dev server with HMR
- `npm run build` — typecheck (`tsc -b`) then production build
- `npm run lint` — oxlint
- `npm run preview` — serve the production build locally
