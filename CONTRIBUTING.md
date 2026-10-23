# Contributing

## Current phase

This repository has buildable application scaffolding, but product features are not implemented. Read the [specification](docs/specification.md) and [feature checklist](docs/features.md) before proposing changes.

## Implementing features

1. Select a bounded unchecked item or a related group from the checklist.
2. Preserve the specified behavior and privacy boundaries. Propose changes to the specification explicitly when needed.
3. Implement the feature and validate it with checks appropriate to the change.
4. Mark an item implemented only when the behavior exists and its acceptance criteria are verified. Scaffolding or a mock screen alone is not completion.
5. Record supporting test or validation evidence in the pull request or commit body where useful.
6. Update related documentation in the same change. Leave partially implemented items unchecked and describe the remaining work.

Deferred items are not part of the agreed first release. Changing a checkbox is not a substitute for agreeing to a scope change.

## Validation

Install dependencies with `make install`, then validate before committing:

```sh
make lint test build
git diff --check
```

Use `make format` for formatting and `make test-race` for concurrency changes. Frontend tests currently mean TypeScript checks, and no behavioral suites exist yet. Add meaningful tests with the first product behavior. See the [development guide](docs/development.md) for commands and the directory structure.

## Commits and publishing

- Keep commits discrete and focused.
- Use a short imperative commit subject, with an optional explanatory body.
- Do not add author or co-author overrides or trailers.
- Push only when explicitly authorized.

## Private data

Never commit credentials, OIDC tokens, SIMKL exports, real viewing history, or database backups. Use synthetic or properly redacted fixtures for tests and examples.

## Third-party data

Verify provider terms, access requirements, attribution, and dataset licenses before integrating a source. The repository license does not relicense external data or artwork.
