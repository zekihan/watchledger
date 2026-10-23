# Development

## Prerequisites

- Go matching `go.mod`.
- Node matching `.node-version`, with npm.
- golangci-lint matching the version in `.github/workflows/ci.yml`.

Run `make install` once to install the locked frontend and lint dependencies. The backend requires PostgreSQL 17 or newer and an OIDC provider. Set the variables in `.env.example` in your shell; the server does not load `.env` automatically. It applies embedded, checksum-verified migrations transactionally on startup. Use a dedicated database.

## Run locally

Use two terminals. Both commands are long-running:

```sh
make dev-api
```

```sh
make dev-web
```

The Go server listens on `127.0.0.1:8080`; Vite normally serves `http://127.0.0.1:5173` and proxies `/healthz` and future `/api` requests to Go.

`GET /healthz` reports process liveness only. It does not validate a database or authenticate users.

## Build and serve

```sh
make build
WATCHLEDGER_WEB_DIR=web/dist ./build/watchledger
```

The second command is long-running and serves the built frontend at `http://127.0.0.1:8080`. Production deployment packaging is not implemented yet.

`WATCHLEDGER_ADDR` overrides the Go listening address. `WATCHLEDGER_WEB_DIR` enables static file serving from a directory. The process does not automatically load `.env`; export settings in your shell when needed. `DATABASE_URL`, `PUBLIC_URL`, OIDC settings, and `SESSION_KEY` are required. The session key is base64 of 32 random bytes and encrypts stored OAuth tokens. Losing it invalidates existing sessions. Register `PUBLIC_URL/auth/callback` at the identity provider. Claim paths use dot-separated object keys and accept a string or array of strings. Login requests `openid profile offline_access`. Refresh must return a validated ID token with current authorization claims or a UserInfo response with the matching subject. Missing claims deny admission or remove admin access. Sessions last at most 12 hours; authorization is refreshed at token expiry or after one hour, whichever is earlier. HTTPS is required except on loopback development URLs.

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
web/src/               React UI bootstrap
web/                   Frontend build, type, and lint configuration
db/migrations/         Future PostgreSQL migrations
db/queries/            Future sqlc source queries
deploy/                Future Compose and Helm packaging
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

Persistence and OIDC foundations are implemented. The React UI now connects core catalog, tracking and list APIs. Several product workflows, live provider validation, packaging, and browser acceptance remain incomplete. Actual identity-provider compatibility still needs deployment configuration. See `implementation-notes.md` for source blockers.

## Database and generated queries

Install sqlc v1.31.1, then run `sqlc generate` after changing `db/queries` or migrations. Commit the generated Go queries. Do not edit applied migration files; add the next numbered migration. The application refuses changed checksums.

Set `TEST_DATABASE_URL` to a disposable PostgreSQL database to run migration and database constraint tests. Never point tests at a production database. Tests use synthetic records and transaction rollback. `/readyz` checks database connectivity; `/healthz` remains process liveness only.
