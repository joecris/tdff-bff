# tdff-bff

Backend-for-Frontend for the tdff React SPA. Owns the Auth0 OAuth/OIDC flow,
holds access/refresh tokens server-side (Redis), issues an opaque
`HttpOnly`/`Secure` session cookie to the browser, and proxies API calls to
the backend with the access token attached. Never exposes tokens to the
browser — see the architecture/security rationale in the planning doc.

Planning doc: `/Users/joecrismolina/.claude/plans/i-want-to-create-proud-dusk.md`
(decisions made, open questions, phased roadmap — read this before making
architectural changes).

## Status

Phase 1 (skeleton) — module scaffold, config loader, `/healthz`. Auth,
sessions, and the API proxy land in later phases per the roadmap above.

## Local development

```bash
cp .env.example .env   # fill in values as later phases require them
make docker-up          # starts Redis
make run                 # starts the BFF on :8080
curl localhost:8080/healthz
```

## Project layout

- `cmd/server` — entrypoint; Vercel's Go Framework Preset auto-detects this path.
- `internal/config` — env var loading/validation, grouped by phase.
- `internal/router` — route composition + middleware chain.
- `internal/auth`, `internal/session`, `internal/store/redis`, `internal/proxy`,
  `internal/middleware`, `internal/handlers` — land in Phases 2–5.

## Deployment

Deploys to Vercel via GitHub Actions (see `.github/workflows/deploy.yml` once
added in Phase 6), using the Go Framework Preset (`vercel.json`). Non-prod
and prod are separate Vercel environments backed by separate Auth0 tenants
and separate Upstash Redis databases.
