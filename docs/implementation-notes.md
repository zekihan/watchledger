# First-release implementation evidence

## Source verification, 2026-10-23

- Repository baseline passes `make lint test build` with Go 1.27.3 and Node 24.22.1.
- AniDB API overview, HTTP definition and policy return HTTP 403 from this environment. Current client registration, rate, cache and attribution requirements remain unverified. AniDB integration is gated; no fallback semantics have been substituted.
- animap release `v2026.10.21` downloaded outside the repository and its published SHA-256 verified. Schema is version 2, generated `2026-10-21T15:06:52Z`, mapping revision `051ec3f7db309c7cc28bccd06535231ed4f13194`, with 16,326 AniDB rows.
- The current animap schema maps TV episodes to TVDB, not TMDB TV. TVDB numbering must not be treated as TMDB numbering. TMDB external-ID resolution and explicit review remain necessary.
- Inspected ordinary, reordered/special and combined-film examples (AniDB 6098, 23 and 28). Explicit versus default-inferred targets are distinguished. Akira's six AniDB parts share TMDB movie 149; this is not permission to propagate progress without user consent.
- Dataset remains external. ODbL 1.0 / DbCL 1.0 attribution and provenance must accompany imported mappings; the application's MIT license does not cover that data.
- TMDB's official FAQ confirms noncommercial attribution requirements. Live TMDB validation remains pending in the operator’s environment.
- Actual OIDC deployment claims remain unavailable. A private SIMKL JSON export was subsequently inspected; see the continuation below. Synthetic tests cannot establish the actual provider's refresh behavior.

References: [animap release](https://github.com/cplieger/animap/releases/tag/v2026.10.21), [schema](https://github.com/cplieger/animap/blob/4681b20094a31dc7d00b52a1f3365e14ae8c6dbc/docs/schema.md), [license](https://github.com/cplieger/animap/blob/4681b20094a31dc7d00b52a1f3365e14ae8c6dbc/LICENSE-DATA), [TMDB FAQ](https://developer.themoviedb.org/docs/faq).

## Persistence and identity foundation

`make lint test build test-race` passes with disposable PostgreSQL 17.11. Migration tests exercise initial application, repeat application, and rejection of a cross-account status foreign key. Current-claim tests cover missing admission, separate admin grants, admin removal at refresh, and mismatched UserInfo subjects. Session ciphertext tampering is rejected. Full browser login and actual deployment-provider compatibility remain unverified; related checklist items remain unchecked.

## Core workflows in progress

Real TMDB adapters cover search, discovery, details, seasons and watch-provider offers; protocol and provider-outage tests use explicitly synthetic HTTP fixtures. Live TMDB acceptance is deferred to the operator’s own environment. Authenticated PostgreSQL HTTP tests cover single/bulk tracking, retained dates, completion, explicit unwatch decisions, cross-account status rejection, smart-list unknown availability and missing references, export/restore repeat application, and non-admin mapping denial. Mapping relationship tests verify revision-bound consent and preservation of source history.

The React UI uses these APIs, with English/Turkish strings, responsive styles and semantic controls. Browser acceptance is blocked: the cloud browser cannot reach the command environment's loopback server and rejects local-file navigation. Frontend jsdom tests are not browser or visual acceptance. No screenshots or desktop/mobile visual acceptance are claimed.

Subsequent increments add TV person-credit candidates, calendar/statistics, background refresh, and packaging. Their implementation and regression checks do not substitute for live-provider, full browser/accessibility, or deployment-smoke acceptance. Anime source integration/alternate views and complete SIMKL migration remain incomplete. Partial and unverified workflows remain unchecked.

## Extended verification

- `internal/auth/login_test.go` exercises a signed synthetic OIDC issuer, PKCE, nonce/state flow, admission before account creation, stable issuer/subject binding through a display-name change, and no first-user admin promotion. `service_test.go` covers current UserInfo claims, missing admission, removed admin access, mismatched subjects, and encrypted-token tampering.
- `internal/httpapi/library_test.go` uses the actual authentication middleware and PostgreSQL. It checks unauthenticated access, cross-account status rejection, known-date preservation, single/season/whole-show tracking, explicit unwatch status handling, custom-status lock/rename/replacement, rating validation, referenced-list deletion and rename stability, repeat restore and user isolation, denied non-admin mapping changes, CSRF origin rejection, and calendar timezone boundaries.
- `internal/tracking/rules_test.go` verifies earlier-episode selection across seasons, specials exclusion, unaired/unknown-release exclusion, and ongoing versus ended completion rules.
- `web/src/App.test.tsx` uses jsdom with clearly synthetic API responses. It verifies OIDC-only login/language switching, Turkish library grid/table behavior, and an earlier-episode dialog that defaults to only the selected episode. These are not browser-layout checks.
- `internal/catalog/cache_test.go` checks cached metadata during provider outages, missing requested-language fallback, and uncached failures. `internal/jobs/runner_test.go` checks persisted retry state and delayed retry after an outage.
- `internal/portable/snapshot_test.go` validates date precision, episode ownership, and cyclic-list rejection. `internal/mapping/mapping_test.go` checks canonical revisions, partial combined targets, consent invalidation, and unchanged source history.
- A PostgreSQL custom-format dump of the disposable synthetic database was restored into a separate empty database; four migration records and synthetic catalog rows were recovered. No real viewing history was used.
- Packaging commit `aec7354cffa5244ec1a357dc104bfad8be9f9855` passed hosted CI run [39891809489](https://github.com/zekihan/watchledger/actions/runs/39891809489), including the actual application-container build and Helm lint/template validation. This does not establish Compose startup or Kubernetes deployment acceptance.

Current source gates remain AniDB API-policy requirements, source episode identity reconciliation, custom-list export coverage, and live TMDB/OIDC deployment configuration. The approved specification is unchanged. Anime-native integration, versioned mapping ingestion and linked alternate views, and faithful SIMKL migration remain incomplete rather than being replaced with a different source or guessed data. Browser acceptance remains blocked by cloud-browser/command-network separation; local-file browser navigation is not allowed. No local-user-computer or external-agent fallback was used.

## Final checkpoint before source-dependent continuation

Feature branch: `feat/first-release`. No merge or homelab deployment was performed. Local `make lint test build test-race` passes with a disposable PostgreSQL instance. The frontend suite has three behavioral tests. Commits through `64fbb8b603a5d726b44c69cdb73141c439d0a830` include the checked implementation and two focused data-safety fixes: background availability refresh does not extend a country's foreground-use window, and concurrent language refreshes do not overwrite a newer cached translation. Missing tracking intent is rejected rather than interpreted as an unwatch.

The application is not first-release complete. Required next inputs are current AniDB API/client/policy documentation, custom-list source coverage and securely configured live TMDB/OIDC test settings in the operator’s environment. Real browser acceptance remains unavailable in the current isolated environment. Container build and Helm rendering are verified; Compose startup and a Kubernetes deployment smoke test are not. Required provider-branding verification also remains open. All of these remain release blockers rather than silently waived checks.

## SIMKL continuation after private format inspection

A private JSON backup established the movies/shows/anime structure, source ID fields, status/rating fields, and explicit season/episode records. No private file, real title, viewing history, credential, or dataset is included in this repository. The observed backup has no custom-list section. This does not prove that SIMKL cannot export lists through another supported route.

The new parser validates document structure and item identities, preserves explicit episode gaps and timestamp/date/unknown precision, and does not infer episode history from summary counts. Duplicate source IDs, invalid ratings/dates, and invalid episode records remain flagged for review. Anime entries never resolve through a shared TMDB ID.

Authenticated SIMKL preview and apply endpoints retain the source JSON privately, require a recent per-user export before application, and add safely resolved cached movie/show library records without changing existing app edits. Movie watch records retain date precision and provenance; explicit app unwatch decisions are respected. Original source IDs and unsupported fields remain in the archive. Uncached identities and all source episode-number mappings stay in a persistent private review queue. They are not silently matched to TMDB/AniDB episodes. Reuploading a backup can re-preview and retry newly cached identities; repeat application does not duplicate history or per-import review rows.

The UI includes preview, explicit apply, archive download, and a read-only unresolved review list. It explicitly identifies partial support. Custom-list import, episode mapping resolution, native anime integration, and editable conflict resolution are not implemented by this increment and remain release blockers. The review list is not a claim of a completed resolution workflow.

Synthetic PostgreSQL HTTP regression tests cover backup enforcement, repeat application, history/date preservation, app-edits-win, explicit unwatch tombstones, anime non-merging, archive retention, and cross-account archive/apply/review denial. Frontend behavioral tests cover explicit application, missing-backup blocking, invalid-file replacement, and stale asynchronous preview results. These do not replace real browser acceptance.

The operator chose to run live API tests later on their own PC. Continue development and validation in the command workspace; do not require live credentials here. See `local-acceptance.md` for the remaining operator checks. The feature branch remains unmerged and no existing infrastructure was changed.
