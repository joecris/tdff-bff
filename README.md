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

Phase 6 (CI/CD) — all prior phases (auth flow, Redis sessions, API proxy,
hardening) verified end-to-end against a real non-prod Auth0 tenant, real
local Redis, and a real locally-running backend. `.github/workflows/ci.yml`
mirrors `tdff-backend`'s QA pipeline (lint, format check, vet, test +
coverage, build, a real-Redis integration job, Snyk) with a gated deploy to
Vercel prod on push to `main`. GitHub secrets, Vercel prod env vars, and
branch protection (PRs required on `main`, all QA jobs must pass, no direct
pushes even for admins) are all configured — this repo's changes now go
through a feature branch + PR. See "CI/CD" and "Deployment" below for the
full one-time setup this needed.

**Hardening (`internal/middleware`)**:
- `RequireCustomHeader` — CSRF defense-in-depth on `/api/*` only. The SPA
  must send `X-Requested-With: XMLHttpRequest` on every proxied call; a
  cross-site form/`<img>` can never set that header, so this forces a CORS
  preflight this same-origin deployment doesn't answer. Not applied to
  `/bff/auth/*`, which are top-level browser navigations (redirects, links)
  that can never carry a custom header anyway.
- `SecurityHeaders` — `X-Content-Type-Options`, `X-Frame-Options`,
  `Referrer-Policy`, a strict `Content-Security-Policy`, `Permissions-Policy`,
  and `Strict-Transport-Security` on every BFF-originated response
  (`/healthz`, `/bff/auth/*`). **Deliberately not applied to `/api/*`**: an
  earlier version applied it globally, which left every proxied response
  with two copies of these headers — ours plus the backend's own, since
  `httputil.ReverseProxy` appends the upstream's headers rather than
  replacing what's already set. The backend owns its own response headers;
  regression-tested in `internal/router`.
- Cookie audit against the RFC checklist: session cookie is `__Host-`
  prefixed, `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, no `Domain`
  attribute. `config.SessionCookieHasRecommendedPrefix` logs a startup
  warning (non-fatal) if `SESSION_COOKIE_NAME` is ever overridden away from
  the `__Host-` prefix.
- Session lifetime (`SESSION_TTL_SECONDS`, default 7 days): a **sliding
  idle-timeout window**, not a hard expiry — confirmed deliberate, not
  Auth0-constrained (the non-prod tenant has no maximum refresh token
  lifetime set, so refresh tokens there are effectively indefinite). Every
  successful token refresh re-saves the session with a fresh TTL, so an
  actively-used session renews itself indefinitely; it only expires after
  this many seconds of *no* requests at all. No absolute session-age cap
  exists on top of this — revisit if that's ever needed.

**Path convention**: `/bff/auth/*` is the only BFF-owned namespace (login,
callback, logout, session). Everything else — `/api/*` — is proxied to the
backend **unchanged**, since the backend already namespaces its own routes
under `/api`; there's deliberately no extra `/bff` nesting on the proxy, to
avoid doubling that prefix.

**Endpoints, for SPA integration**:
- `GET /bff/auth/login?returnTo=<path>` — starts login. `returnTo` is
  optional; if given, must be a same-origin-relative path starting with a
  single `/` (an absolute URL, `//host/...`, or anything with a backslash
  is silently dropped, not errored — falls back to `PostLoginRedirectURL`).
  Send the SPA's current route here so login round-trips back to where the
  user actually was, not always the home page. **Must be a real browser
  navigation** (`window.location.href = ...`), not a `fetch` — it needs to
  carry the browser through the Auth0 redirect chain.
- `GET /bff/auth/logout` — same: a real navigation, not `fetch`.
- `GET /bff/auth/session` — `{authenticated, email}`. Safe to `fetch` with
  `credentials: 'include'` on app load / route change to check auth state.
- `/api/*` — proxied backend calls. `fetch` with `credentials: 'include'`
  and a `X-Requested-With: XMLHttpRequest` header (required — see
  "Hardening" above; omit it and every proxied call gets a 403).

**Test coverage**: `make coverage` enforces a 75% threshold on `internal/...`
(currently ~87%; `cmd/server` is excluded — it's composition-root wiring with
no logic to unit test independent of an actual process boot). Network-bound
code (Auth0 OIDC discovery/token exchange, the backend API) is tested against
fakes — a minimal in-process OIDC provider (`internal/auth`, signed with a
throwaway RSA key via `go-jose`) and an `httptest` server — rather than a
live Auth0 tenant, so CI doesn't need real credentials to enforce coverage.

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

Other useful targets: `make test` (race detector), `make coverage`
(threshold-enforced), `make test-integration` (needs a real Redis at
`REDIS_URL`, e.g. from `make docker-up`), `make fmt-check` / `make lint` /
`make vet` (same checks CI runs).

## Project layout

- `cmd/server` — entrypoint; Vercel's Go Framework Preset auto-detects this path.
- `internal/config` — env var loading/validation, grouped by phase.
- `internal/router` — route composition + middleware chain.
- `internal/auth`, `internal/session`, `internal/store/redis`,
  `internal/handlers`, `internal/proxy`, `internal/middleware` — Phases
  2–5, done.
- `.github/workflows/ci.yml` — Phase 6, done (see "CI/CD" below).

## CI/CD

`.github/workflows/ci.yml` runs on every PR and push to `main`: lint
(`golangci-lint`), `gofmt` check, `go vet`, unit tests + coverage (75%
threshold via `make coverage`), a build, a real-Redis integration job
(`redis:7-alpine` service container — independent of `docker-compose.yml`,
which is local-dev-only), and a Snyk dependency scan. `deploy` runs only
after every job above passes, only on a push to `main`.

**One-time setup this repo needs** (not in the workflow file):

1. In Snyk: reuse the existing service account token from `tdff-backend`
   (org Settings → Service Accounts) — no new Snyk account needed.
2. In GitHub (this repo's Settings → Secrets and variables → Actions), add:
   - `SNYK_TOKEN` — same value as above.
   - `VERCEL_TOKEN`, `VERCEL_ORG_ID`, `VERCEL_PROJECT_ID` — see "Deployment"
     below for how to get the latter two.
3. (Recommended) Settings → Branches: require the CI jobs to pass before
   merging into `main`.
4. (Recommended, free) Settings → Code security and analysis: enable
   Dependabot alerts + security updates.

The Snyk gate starts at `--severity-threshold=high` (a fresh dependency
tree almost always has some low/medium transitive noise) — tighten once a
real baseline is known.

## Deployment

Prod domains: BFF `https://tdff-bff.vercel.app`, SPA `https://tdff-app.vercel.app`,
backend API `https://tdff-backend.vercel.app` (already live).

**Important**: `vercel.app` is on the Public Suffix List, so the SPA and BFF
subdomains are fully separate origins to the browser — the same-origin
rewrite below isn't an optimization, it's what makes cookies/CSRF work at
all here. `AUTH0_CALLBACK_URL` therefore points at the **SPA's** domain, not
the BFF's own: the browser needs to land back on the origin it's already on
so the session cookie ends up there too, not on a domain the SPA can't read
from.

**Vercel project (this repo)**:
- Framework preset: Go (already set in `vercel.json`).
- Do **not** connect Vercel's GitHub App/native Git integration — deploys
  go through the CLI from GitHub Actions only (`vercel deploy --prod` in
  the `deploy` job). Connecting the native integration on top would also
  auto-deploy a preview on every push/PR, which isn't wanted (see the
  plan's "Non-prod BFF" decision — deferred until there's a non-prod
  backend/SPA to pair it with).
- Get `VERCEL_ORG_ID`/`VERCEL_PROJECT_ID` by running `vercel link` once
  locally against the project (writes `.vercel/project.json`, gitignored)
  and reading the values out of that file.
- Environment Variables (Project Settings → Environment Variables,
  Production scope — `PORT` is injected by Vercel, don't set it):

  ```
  APP_ENV=prod
  AUTH0_DOMAIN=<prod tenant domain>
  AUTH0_CLIENT_ID=<prod app client id>
  AUTH0_CLIENT_SECRET=<prod app client secret>
  AUTH0_AUDIENCE=<prod API identifier, matching the backend's configuration>
  AUTH0_CALLBACK_URL=https://tdff-app.vercel.app/bff/auth/callback
  AUTH0_LOGOUT_REDIRECT_URL=https://tdff-app.vercel.app/
  REDIS_URL=<Upstash prod rediss:// connection string>
  BACKEND_API_BASE_URL=https://tdff-backend.vercel.app
  LOG_LEVEL=info
  ```

**Upstash**: one Redis database (prod), region matched to wherever the
Vercel functions run. Use the plain `rediss://` TCP connection string (not
the REST API variant — this app uses standard `go-redis`, not the REST
client, since the Go Framework Preset runs a real long-lived process
rather than the Edge Runtime the REST client is meant for).

**Auth0**: a separate prod tenant + Application (Regular Web App, Refresh
Token grant enabled) + API (Allow Offline Access enabled) — same shape as
the working non-prod one, see above. Allowed Callback URLs =
`https://tdff-app.vercel.app/bff/auth/callback`; Allowed Logout URLs =
`https://tdff-app.vercel.app/`.

**SPA repo** (not this repo, but a hard dependency — hand this to whoever
owns it): a `vercel.json` rewrite so the browser only ever talks to
`tdff-app.vercel.app`:

```json
{
  "rewrites": [
    { "source": "/bff/:path*", "destination": "https://tdff-bff.vercel.app/bff/:path*" },
    { "source": "/api/:path*", "destination": "https://tdff-bff.vercel.app/api/:path*" }
  ]
}
```

And every fetch/XHR call the SPA makes to `/api/*` must send
`X-Requested-With: XMLHttpRequest` — the CSRF defense-in-depth header
`internal/middleware.RequireCustomHeader` requires (see "Hardening" above).
