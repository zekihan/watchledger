# Watchledger

A self-hosted movie, TV, and anime tracker with private user libraries, episode progress, streaming availability, and portable viewing history.

## Project status

**Current release: 0.1.4. Early release; product acceptance remains incomplete.**

The first release is defined in the [application specification](docs/specification.md). Track implementation in the [feature checklist](docs/features.md); partial or unverified items remain unchecked.

Implemented workflows include private PostgreSQL-backed tracking, OIDC admission/admin claims, source-aware anime alternate views and statistics, custom statuses, manual/nested smart lists, calendar, portable export/restore, SIMKL previews/review and optional read-only custom-list fetching. English/Turkish screens include source-stable season/episode links and installable-app assets. TMDB and AniDB metadata are cached with bounded background refresh; native title-index search, Hot Anime discovery and cached creator credits keep their own source identities.

Automated backend, database, race and frontend behavioral tests support these increments; they are not a production-readiness claim. Actual private import coverage, additional native-source coverage decisions, configured-provider compatibility and real-browser/deployment acceptance remain open. Work requiring your involvement is collected in [TODO.md](TODO.md), with detailed [implementation evidence](docs/implementation-notes.md) and a [deployment guide](docs/deployment.md).

## Install version 0.1.4

Container images: `ghcr.io/zekihan/watchledger:0.1.4` and `docker.io/zekihan/watchledger:0.1.4` (Linux amd64/arm64). Helm chart: `oci://ghcr.io/zekihan/charts/watchledger`, version `0.1.4`.

Use the [deployment guide](docs/deployment.md) for external PostgreSQL, OIDC and secret configuration. Read the [release notes](docs/releases/0.1.4.md) for known limitations and [release process](docs/releasing.md) for publishing and verification.

## Local development

Install the prerequisites listed in the [development guide](docs/development.md), then run:

```sh
make install
make lint test build
```

Run `make dev-api` and `make dev-web` in separate terminals for the long-running development servers. Configure PostgreSQL, OIDC, public origin and session encryption as described in `.env.example`; keep development servers on loopback.

## First-release scope

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

## Stack

- Go HTTP backend with `pgx` and `sqlc`.
- React, TypeScript, Vite, Tailwind CSS, accessible UI primitives, and TanStack Query.
- PostgreSQL for application state, metadata caching, and background jobs.
- One app container serving the frontend and API and running background jobs, with PostgreSQL alongside it or provided externally.

Database migrations and sqlc queries are committed. Background refresh runs in the application process with PostgreSQL job state; no extra worker service is used.

## Documentation

- [Application specification](docs/specification.md): agreed behavior, boundaries, and acceptance criteria.
- [Feature checklist](docs/features.md): implementation status for the complete agreed release and explicitly deferred work.
- [Data sources](docs/data-sources.md): provider integrations, verified constraints, and open verification tasks.
- [Development guide](docs/development.md): directory structure, prerequisites, and local commands.
- [Contributing](CONTRIBUTING.md): development and feature-completion conventions.
- [Agent instructions](AGENTS.md): repository rules shared through the `CLAUDE.md` symlink.

## Data ownership and third-party services

Viewing history belongs in the self-hosted database and must be exportable. The app uses external catalog and availability providers; cached library data and tracking should continue working when providers are unavailable.

The integrated catalog/mapping sources are TMDB, AniDB, and animap. AniDB API requirements were verified from operator-supplied official documentation; its data is non-commercial CC-BY-NC-SA 4.0. SIMKL is a migration source, not an ongoing synchronization dependency.

The MIT license covers this repository's original code and documentation. It does not grant rights to third-party metadata, artwork, trademarks, or mapping datasets. Provider attribution and applicable data licenses must be respected; see [data sources](docs/data-sources.md).

## License

[MIT](LICENSE). Copyright 2026 Zekihan AZMAN.
