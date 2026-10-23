# Development

## Prerequisites

- Go matching `go.mod`.
- Node matching `.node-version`, with npm.
- golangci-lint v2.14.0.

Run `make install` once to install the locked frontend and lint dependencies. No database or identity provider is needed for the current bootstrap.

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

The second command is long-running and serves the built frontend at `http://127.0.0.1:8080`. Client-side routing and production deployment are not implemented yet.

`WATCHLEDGER_ADDR` overrides the Go listening address. `WATCHLEDGER_WEB_DIR` enables static file serving from a directory. The process does not automatically load `.env`; export settings in your shell when needed. Commented provider settings in `.env.example` are reserved and are not read yet.

## Checks

```sh
make lint test build
make test-race
```

The Go commands target `cmd` and `internal` explicitly so dependencies under `node_modules` are not treated as Go application packages. Markdown linting uses Git-aware file discovery and skips symlink duplicates.

`make lint` runs Go linting, type-aware frontend ESLint, Markdown linting, and formatting checks. `make test` runs Go tests and TypeScript checks; there are no behavioral test suites in this skeleton. Add real tests alongside product behavior. `make format` applies Go and Prettier formatting.

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

The skeleton runs a liveness endpoint and a placeholder React page. It does not implement authentication, tracking, metadata, persistence, deployment packaging, or the localized product UI. All product checklist entries remain unchecked.
