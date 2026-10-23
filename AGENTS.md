# Repository instructions

## Product contract

- Read `docs/specification.md` before changing product behavior.
- Use `docs/features.md` as the implementation tracker. Scaffolding is not a completed product feature.
- Keep changes scoped to the request. Propose material specification changes explicitly.
- Preserve history and source provenance; never guess dates or silently merge anime entries.
- Keep user data private and enforce authorization server-side.
- OIDC access and admin roles come from separate configured claims. Never promote the first user automatically.
- Do not introduce infrastructure services or external integrations outside the agreed scope.

## Layout

- `cmd/watchledger/`: process entrypoint and lifecycle.
- `internal/httpapi/`: HTTP routing and transport concerns.
- `internal/`: future private domain, provider, store, and job packages, created as implemented.
- `web/src/`: React UI and client-side state.
- `db/migrations/`: ordered PostgreSQL migrations.
- `db/queries/`: source queries for future sqlc generation.
- `deploy/`: future Compose and Helm packaging.
- `docs/`: specification, feature status, sources, and development guidance.
- `.github/workflows/`: validation automation; no automatic publishing.

## Commands and checks

- Use Go as declared by `go.mod` and Node as declared by `.node-version`.
- Run `npm ci` before frontend or documentation checks.
- Run `make lint test build` before committing. Use `make test-race` for changes to concurrent Go behavior.
- Format with `make format`; do not disable lint rules to hide problems.
- Write meaningful tests for new behavior, especially authorization, history, mapping, filters, and imports. Do not add tests that merely mirror trivial scaffolding.
- Frontend `make test` currently checks types only; there is no frontend behavioral test suite yet. State this limitation accurately.
- Do not call database readiness or product features complete because `/healthz` succeeds.

## Implementation conventions

- Use standard Go formatting, explicit error handling, contexts for external operations, and structured logging without credentials.
- Keep business rules separate from HTTP handlers and provider adapters.
- Use TypeScript strictness and type-aware ESLint; avoid `any`, floating promises, and unsafe assertions.
- Use accessible controls and preserve the planned English/Turkish localization boundary.
- Prefer PostgreSQL and the agreed in-process job model; do not add Redis or a separate queue by default.
- Add pgx/sqlc and their generation configuration with real schema/query work rather than placeholder generated APIs.
- Add packages when needed, not empty files for every planned feature.

## Data and credentials

- Never commit secrets, tokens, real imports, viewing history, database dumps, or generated local state.
- Keep examples synthetic and credentials clearly illustrative.
- Check source access rules and data licenses before integration. MIT does not relicense third-party metadata or artwork.
- Back up before applying imports. Do not delete, overwrite, or reinterpret history silently.

## Git workflow

- Make discrete, logically separated commits with short imperative subjects and optional bodies.
- Never add author/co-author overrides or trailers.
- Run the relevant lint, tests, and build before committing.
- Never push unless explicitly requested. Never force-push or delete branches without explicit instruction.
- Keep `CLAUDE.md` as a relative symlink to this file so agent instructions have one source of truth.
