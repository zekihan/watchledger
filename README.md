# Watchledger

A self-hosted movie, TV, and anime tracker with private user libraries, episode progress, streaming availability, and portable viewing history.

## Project status

**Bootstrap skeleton. No product features are implemented yet.**

The first release is defined in the [application specification](docs/specification.md). Track implementation in the [feature checklist](docs/features.md); every item starts unchecked.

The repository includes a Go liveness server, a placeholder React page, shared lint/build commands, and CI configuration. Authentication, persistence, tracking, container packaging, and deployment charts are not implemented.

## Local development

Install the prerequisites listed in the [development guide](docs/development.md), then run:

```sh
make install
make lint test build
```

Run `make dev-api` and `make dev-web` in separate terminals for the long-running development servers. The skeleton has no authentication; keep it on loopback.

## Planned first release

- Private libraries for multiple users, with OIDC login and claim-based access and admin roles.
- Movie, TV, anime, season, episode, cast, and crew browsing.
- Watched history, episode progress, personal ratings, and custom tracking statuses.
- Country-specific streaming availability and discovery filters.
- Manual lists and nested smart lists with dependency tracking.
- Anime-native and show/season views over shared viewing history.
- SIMKL migration with preview, conflict handling, and an unresolved-item review queue.
- A polished responsive web interface in English and Turkish.
- Up next, a release calendar, statistics, and portable per-user export and restore.
- Docker Compose and Helm deployment options.

## Planned stack

- Go HTTP backend with `pgx` and `sqlc`.
- React, TypeScript, Vite, Tailwind CSS, accessible UI primitives, and TanStack Query.
- PostgreSQL for application state, metadata caching, and background jobs.
- One app container serving the frontend and API and running background jobs, with PostgreSQL alongside it or provided externally.

The Go/React build and lint tooling is bootstrapped. pgx/sqlc and database jobs remain planned until real persistence is added.

## Documentation

- [Application specification](docs/specification.md): agreed behavior, boundaries, and acceptance criteria.
- [Feature checklist](docs/features.md): implementation status for the complete agreed release and explicitly deferred work.
- [Data sources](docs/data-sources.md): proposed providers, verified constraints, and open verification tasks.
- [Development guide](docs/development.md): directory structure, prerequisites, and local commands.
- [Contributing](CONTRIBUTING.md): development and feature-completion conventions.
- [Agent instructions](AGENTS.md): repository rules shared through the `CLAUDE.md` symlink.

## Data ownership and third-party services

Viewing history belongs in the self-hosted database and must be exportable. The app will use external catalog and availability providers; cached library data and tracking should continue working when providers are unavailable.

The proposed catalog sources are TMDB, AniDB, and animap. AniDB API access requirements must be verified before implementation. SIMKL is a migration source, not an ongoing synchronization dependency.

The MIT license covers this repository's original code and documentation. It does not grant rights to third-party metadata, artwork, trademarks, or mapping datasets. Provider attribution and applicable data licenses must be respected; see [data sources](docs/data-sources.md).

## License

[MIT](LICENSE). Copyright 2026 Zekihan AZMAN.
