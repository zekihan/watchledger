# Development

## Prerequisites

- Go matching `go.mod`.
- Node matching `.node-version`, with npm.
- golangci-lint matching the version in `.github/workflows/ci.yml`.

Run `make install` once to install the locked frontend and lint dependencies. The backend requires PostgreSQL 17 or newer with a UTF8-encoded database and an OIDC provider. Non-UTF8 databases are rejected at startup so Unicode validation and character limits remain consistent. Set the variables in `.env.example` in your shell; the server does not load `.env` automatically. It applies embedded, checksum-verified migrations transactionally on startup. Use a dedicated database.

## Run locally

Use two terminals. Both commands are long-running:

```sh
make dev-api
```

```sh
make dev-web
```

The Go server listens on `127.0.0.1:8080`; Vite normally serves `http://127.0.0.1:5173` and proxies `/api`, `/auth`, `/healthz` and `/readyz` requests to Go. For this two-terminal setup, set `PUBLIC_URL=http://127.0.0.1:5173` and register that origin’s `/auth/callback` at your test identity provider so login returns to Vite and write-origin checks match. Use the same host spelling in your browser; `localhost` and `127.0.0.1` are different origins. For the built single-process app, keep `PUBLIC_URL` on the Go server’s external origin instead.

`GET /healthz` reports process liveness only. It does not validate a database or authenticate users.

## Build and serve

```sh
make build
WATCHLEDGER_WEB_DIR=web/dist ./build/watchledger
```

The second command is long-running and serves the built frontend at `http://127.0.0.1:8080`. See `deployment.md` for Compose and Helm packaging and its outstanding smoke checks.

`WATCHLEDGER_ADDR` overrides the Go listening address. `WATCHLEDGER_WEB_DIR` enables static file serving from a directory. The process does not automatically load `.env`; export settings in your shell when needed. `DATABASE_URL`, `PUBLIC_URL`, OIDC settings, and `SESSION_KEY` are required. The session key is base64 of 32 random bytes and encrypts stored OAuth tokens. Losing it invalidates existing sessions. Register `PUBLIC_URL/auth/callback` at the identity provider. Claim paths use dot-separated object keys and accept a string or array of strings. Login always requests `openid profile offline_access`; `OIDC_EXTRA_SCOPES` adds space-separated provider scopes without duplicates. `OIDC_CLAIMS_SOURCE=auto` preserves ID-token authorization at login and uses validated refreshed ID-token claims when available, otherwise subject-validated UserInfo. Set `OIDC_CLAIMS_SOURCE=userinfo` to explicitly use current subject-validated UserInfo for authorization at both login and refresh; returned ID tokens are still validated. Claims from different sources are never merged. For Authelia group-based admission, allow the `groups` scope on the provider client, set `OIDC_EXTRA_SCOPES=groups`, and select `OIDC_CLAIMS_SOURCE=userinfo` when group claims are supplied only through UserInfo. The [Authelia claims documentation](https://www.authelia.com/integration/openid-connect/openid-connect-1.0-claims/) describes its default claim locations. Refresh must return a validated ID token with current authorization claims or a UserInfo response with the matching subject. Missing claims deny admission or remove admin access. Sessions last at most 12 hours; authorization is refreshed at token expiry or after one hour, whichever is earlier. HTTPS is required except on loopback development URLs.

## TypeScript compiler and lint API

Type checking and production builds use the TypeScript 7 native `tsc` executable, installed through the `@typescript/native` npm alias. ESLint retains type-aware rules using `typescript`, which aliases the official `@typescript/typescript6` compatibility package. TypeScript 7 does not yet expose the compiler API required by typescript-eslint.

This follows [Microsoft's side-by-side setup](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/#running-side-by-side-with-typescript-60). Keep both aliases until typescript-eslint supports the native compiler API. Do not bypass peer checks or disable type-aware linting to upgrade the compiler.

## Checks

```sh
make lint test build
make test-race
```

The Go commands target `cmd` and `internal` explicitly so dependencies under `node_modules` are not treated as Go application packages. Markdown linting uses Git-aware file discovery and skips symlink duplicates.

`make lint` runs Go linting, type-aware frontend ESLint, Markdown linting, and formatting checks. `make test` runs Go tests, TypeScript checks, and frontend behavioral tests; frontend behavioral tests now run through Vitest/jsdom. Add real tests alongside product behavior. `make format` applies Go and Prettier formatting.

The root `katex` override keeps markdownlint's transitive math renderer on a patched release. Review the override when upgrading markdownlint and remove it when the upstream dependency already enforces a patched version.

If a restricted environment denies writes to your default caches, use writable task-local paths for `GOCACHE`, `GOLANGCI_LINT_CACHE`, and `npm_config_cache`; do not change ownership or permissions of unrelated directories to work around that.

## Directory structure

```text
.github/workflows/     Local-equivalent CI checks
scripts/               Repository tooling
cmd/watchledger/       Go entrypoint and graceful shutdown
internal/httpapi/      HTTP routing and process liveness
web/src/               React product screens and jsdom tests
web/                   Frontend build, type, and lint configuration
db/migrations/         Embedded, versioned PostgreSQL migrations
db/queries/            sqlc source queries
deploy/                Compose and Helm packaging
docs/                  Product contract, tracker, sources, development
AGENTS.md              Shared agent instructions
CLAUDE.md              Relative symlink to AGENTS.md
```

Create domain, provider, store, and job packages under `internal/` when implemented. Keep domain logic independent of transport and provider formats.

## Dependency updates

`renovate.json` extends the shared `zekihan/renovate-config` preset used by the other projects. It covers npm workspace dependencies and overrides, Go modules/toolchain, GitHub Actions and their supported tool-version inputs, and `.node-version` through Renovate's built-in managers. Node runtime updates stay within the declared Node 24 support range.

The shared preset supplies the dashboard, labels, reviewer, and Friday 00:00–05:59 Europe/Istanbul update window. Weekly lockfile maintenance uses the same window. This repository explicitly disables automatic merging, including inherited package rules, so updates require review.

Configuration alone does not activate a bot. The Renovate GitHub App must have access to this repository, or a self-hosted Renovate runner must include it. No app installation, runner change, or live bot run is performed by adding this file.

## Current limitations

Persistence and OIDC foundations are implemented. The React UI now connects core catalog, tracking and list APIs. Several product workflows, live provider validation and browser acceptance remain incomplete. Actual identity-provider compatibility still needs deployment configuration. See `implementation-notes.md` for source blockers.

## Database and generated queries

Install sqlc v1.31.1, then run `sqlc generate` after changing `db/queries` or migrations. Commit the generated Go queries. Do not edit applied migration files; add the next numbered migration. The application refuses changed checksums.

Set `TEST_DATABASE_URL` to a disposable PostgreSQL database to run migration and database constraint tests. Never point tests at a production database. Tests use synthetic records with transaction rollback or scoped cleanup. Use a disposable database even when cleanup is expected. `/readyz` checks database connectivity; `/healthz` remains process liveness only.

## AniDB native metadata

Configure the registered public HTTP client identity using ANIDB_CLIENT and ANIDB_CLIENT_VERSION; these are not an AniDB account password or secret token. The operator verified watchledger version 1. Leaving both unset disables uncached live access while preserving cached reads. Compose forwards these variables; Helm exposes anidb.client and anidb.version.

The shared database enforces a two-second provider request interval and a 24-hour per-anime request cooldown, including failed attempts and manual refresh. XML ban responses suspend all provider requests for 24 hours. Do not remove these limits to retry. Cached metadata remains available with a stale warning. Source episodes keep their stable IDs through renumbering; disappearing IDs retire instead of deleting history.

Search supports the official title-only index, exact AniDB IDs and a clearly labelled cached-title fallback. An admin uploads the official XML or gzip file from the mapping/admin screen. Replacement is validated and atomic, with checksum and import timestamp; it creates no catalog metadata or viewing history. Direct download returned HTTP 403 during validation, but an operator-provided file verified the actual format. Download the source at most once per day, and do not treat index import time as its source publication date. No HTML scraping is used. Native artwork and availability are not guessed from TMDB IDs.

Tests use synthetic XML and HTTP servers. To validate an already-downloaded operator response without re-requesting it, set ANIDB_XML_FIXTURE to that local XML file when running the AniDB package tests. Never commit the response. AniDB data retains its own CC-BY-NC-SA 4.0 restrictions and attribution; the application MIT license does not relicense it.

## Synthetic real-browser CI checks

The existing disposable Compose smoke run now invokes `npm run test:browser` using pinned Playwright. Isolated CI matrix jobs select Chromium, Firefox or WebKit through WATCHLEDGER_TEST_BROWSER; each receives its own disposable database and synthetic sessions. Core workflow cases exercise the built app and real PostgreSQL APIs at English/Turkish desktop and phone-sized viewports. Separate catalog-presentation cases use explicitly synthetic provider responses and artwork while retaining real private sessions; they do not claim live-provider acceptance. The suite includes synthetic admission/claim-revocation browser callbacks with a verified ephemeral issuer CA, bulk date preservation and custom-status replacement. The suite covers portable restore preview/apply, spoiler hiding, unknown-date episode tracking, earlier-episode cancellation/default/focus return, manual/smart lists, navigation overflow, header-search keyboard access and private export exclusions.

The fixture requires `GITHUB_ACTIONS=true`, `WATCHLEDGER_SYNTHETIC_OIDC=1` and the ephemeral `SMOKE_DIR` established by `scripts/compose-smoke.sh`. It cannot target an arbitrary origin or operator database. Browser authentication uses only the synthetic session produced by the existing TLS-verified OIDC HTTP flow; it does not bypass certificate warnings, import real cookies or claim browser-level real-provider login acceptance. Trace/video/storage-state capture is disabled. Only synthetic screenshots are uploaded for seven days; no cookie files, keys, database dumps or portable exports are artifacts.

`npm run typecheck:browser` runs with normal tests. `npx playwright test --list` can inspect the suite without starting a browser or fixture. The command workspace cannot launch Chromium under its current process/socket permissions; actual browser success must be established by the exact CI run. Physical Android touch/install, real provider claims and operator data remain separate. Installation follows the [official Playwright CI guidance](https://playwright.dev/docs/ci).
