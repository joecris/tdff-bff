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

Phase 3 (session + Redis) — Auth0 Authorization Code + PKCE login, callback,
logout, and `/bff/auth/session`, verified end-to-end against a real non-prod
Auth0 tenant, with sessions persisted in Redis (`internal/store/redis`,
standard go-redis over TCP+TLS — verified against both a real local
`redis-server` and, via `miniredis`, in unit tests without needing Docker).
`internal/store/memory` remains only as a dependency-free test double; it's
no longer wired into `main.go`. The API proxy lands in Phase 4.

Auth0 app setup notes (non-prod tenant already configured this way — carry
these over when the prod tenant/app is provisioned):
- Application Type: **Regular Web Application**, with the **Refresh Token**
  grant type enabled.
- The API (`AUTH0_AUDIENCE`) has **Allow Offline Access** enabled — required
  for Auth0 to issue a `refresh_token` alongside the access token.

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
- `internal/auth`, `internal/session`, `internal/store/redis`,
  `internal/handlers` — Phases 2–3, done.
- `internal/proxy`, `internal/middleware` — land in Phases 4–5.

## Deployment

Deploys to Vercel via GitHub Actions (see `.github/workflows/deploy.yml` once
added in Phase 6), using the Go Framework Preset (`vercel.json`). Non-prod
and prod are separate Vercel environments backed by separate Auth0 tenants
and separate Upstash Redis databases.
