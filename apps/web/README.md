# @doclens/web

Next.js 15 web client for DocLens. App Router, Tailwind, Clerk auth.

## Routes

| Path            | Group         | Auth     | Notes                                    |
| --------------- | ------------- | -------- | ---------------------------------------- |
| `/`             | `(marketing)` | Public   | Landing page                             |
| `/sign-in/*`    | _root_        | Public   | Clerk-hosted sign-in                     |
| `/sign-up/*`    | _root_        | Public   | Clerk-hosted sign-up                     |
| `/library`      | `(app)`       | Required | M2 stub: renders identity from `/v1/me`  |
| `/library/[id]` | `(app)`       | Required | M5: reader (PDF + Markdown side by side) |
| `/upload`       | `(app)`       | Required | M3: dropzone                             |
| `/search`       | `(app)`       | Required | M7: full-text search                     |

`src/middleware.ts` gates the protected segments via Clerk.

## Local development

1. Copy `.env.example` to `.env.local` and fill in your Clerk keys.
2. From the repo root, run `pnpm install`.
3. Start the API (`make up` from the repo root, then `cd apps/api && go run ./cmd/api`).
4. From this directory, run `pnpm dev`.

The web app expects the API at `INTERNAL_API_URL` (server-side) and
`NEXT_PUBLIC_API_URL` (browser).

## Scripts

- `pnpm dev` — Next dev server
- `pnpm build` — Production build
- `pnpm lint` — ESLint (root flat config + Next presets)
- `pnpm typecheck` — `tsc --noEmit`
- `pnpm test` — Vitest in `--run` mode
