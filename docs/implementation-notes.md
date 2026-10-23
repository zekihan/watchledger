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

## Explicit SIMKL episode review

The next increment adds a private review editor for one-to-one source episode links. Every source episode requires an explicit target choice; equal episode numbers do not initialize a selection. Previously confirmed links are displayed and locked. Confirmation explicitly preserves existing application status, rating, history, and unwatch decisions. The editor paginates long source episode lists and rejects incomplete or duplicate target choices.

Migration 005 stores confirmed per-user source episode identities independently of watched flags. Reimports cannot move a source episode to another target silently, even after it has been unwatched. Server-side validation requires target ownership by the exact explicitly identified catalog title, a recent backup export, and confirmation. Split/combined targets, missing native metadata, and invalid source records remain unresolved rather than being forced through this one-to-one workflow.

Portable exports now include optional `simkl_links`; older version-1 exports without the field still validate. Restore validates source keys and episode ownership. Conflicting existing links reject the restore transaction rather than relocating history. Synthetic PostgreSQL tests cover partial-apply rollback, explicit confirmation, duplicate/foreign targets, exact dates, unknown dates, reimport immutability, unwatch preservation, account isolation, and link/tombstone export/restore. Frontend tests verify empty initial choices, explicit confirmation, duplicate-target blocking, fixed previously confirmed links, and backup gating. Real browser acceptance remains an operator check.

Earlier references to a read-only review list describe the preceding checkpoint. Ordinary cached movie/show conflicts can now be resolved by explicitly keeping app edits, and known episode identities can be explicitly linked. This does not complete AniDB integration, split/combined anime mappings, custom-list source coverage, or live-provider acceptance.

## Resolving uncached source identities

An authenticated review can now load the exact TMDB movie/show ID present in its source record. Metadata loading is a separate action and never inserts library/history records. Target seasons must be selected from the provider's own season list; a source season number does not choose a provider season. Provider failures are retryable and cached/stale responses are identified.

After loading metadata, explicit review confirmation can add a previously unresolved library record and its supported history without reuploading the backup. Existing application values still win. Native anime records without verified AniDB metadata and records without a supported explicit identity remain unresolved; no title-search match or shared anime TMDB ID is assumed. Synthetic HTTP-provider tests cover an uncached movie, differently numbered show episodes, outage/retry, no history during metadata fetch, cross-account fetch denial, and refusal to substitute a shared TMDB ID for native anime. Live credentials are not used by those tests.

## Interrupted tracking flows

Tracking choices now distinguish cancellation from an explicit negative/default choice. Cancel and Escape abort the pending tracking operation rather than applying “only this episode” or retaining status while still unwatching. Leaving the title page cancels a waiting choice, and a preview response arriving after navigation cannot open a dialog or submit history. Explicitly saving the default choice still affects only the selected episode.

Frontend regressions cover Cancel, Escape, navigation while a prompt is open, delayed previews after navigation, and global shortcuts inside the modal. Shortcuts also ignore editable controls; focused movie tracking buttons support the same W action as episode buttons. These behavioral tests do not replace the remaining native browser/focus and mobile acceptance checks.

## Saved list browsing and rule controls

Saved manual and smart lists have a contents view separate from editing. Manual results preserve member order; smart results use the saved rule/sort, private-library scope, and the same unknown-value semantics as preview. Account-scoped result endpoints reject other users' list IDs. Edited previews substitute the proposed rule into the dependency graph so indirect cycles are rejected before save.

The rule builder provides named choices for content kinds, base/custom statuses, existing lists, cached genres, and configured provider options. Numeric operators are limited to numeric fields; empty or pending previews cannot be saved. Manual ordering supports localized up/down buttons alongside dragging, and editing is disabled during save. Result evaluation batches cached availability in the library query and selects metadata deterministically in the user's preferred language instead of choosing a random translation.

Synthetic PostgreSQL tests cover manual order/localization, saved smart results, unknown versus known-empty availability, deterministic metadata filters, edited dependency cycles, and unauthorized/cross-account reads. Frontend tests cover contents browsing, stable IDs behind named selectors, and keyboard-friendly member reordering. Browser layout/accessibility acceptance is still separate.

## Custom-list file import checkpoint

A separate upload supports the documented SIMKL custom-list contents response, including complete multi-page bundles. It does not invent a custom-list section in the observed watchlist backup and makes no live SIMKL requests. Preview reports identity conflicts and missing private-library members. Applying requires a recent export; unresolved membership archives the whole list for review without creating a misleading partial list or changing viewing history.

Successful imports create manual snapshots in source response order, retaining source identity and the unmodified private archive. Reimport preserves current app edits, removed members, and deleted lists. Portable exports/restores retain source identity and deletion markers, preventing duplicate lists or resurrected deletions. Automatic source rules are archived, not translated into guessed smart rules.

Synthetic parser, HTTP/PostgreSQL, portability and frontend tests exercise pagination, source errors, backup gating, private identity resolution, reimport, ordering, app edits/deletions, and export/restore. PostgreSQL must use UTF8 so Unicode name limits behave consistently; startup rejects incompatible encodings. This remains file-based partial migration, not live API or real-browser acceptance.

## Statistics precision and metadata selection

Statistics select the user's preferred metadata language deterministically. Watched episode runtime uses episode metadata only; missing or zero runtimes remain unknown rather than substituting a whole-title duration. Dashboard monthly activity now uses the same account timezone as detailed statistics, preserving date-only values and excluding unknown watched dates. Retired episodes no longer appear in upcoming dashboard results.

Synthetic PostgreSQL regression tests cover month-boundary timestamps in Europe/Istanbul, unchanged date-only records, unknown dates/runtime, translated genre names, account isolation, and retired upcoming episodes. Native browser and live metadata acceptance remain separate.

## Linked-view projection

Cached episode mappings now have a private alternate-view endpoint and interface. Projection is computed in both directions from recorded source events only, never recursively from projected flags. Split/combined relationships require revision-bound consent. Unreviewed, overlapping, retired-source or invalid mappings stay blocked; missing coverage remains visible as incomplete. Source statuses/ratings remain separate and no aggregate rating or watched date is invented.

Synthetic domain and HTTP/PostgreSQL tests cover split/combined directionality, partial coverage, consent, account isolation, retired source identity, and preservation of recorded events. The initial view is read-only; native catalog ingestion and cross-view editing remain separate implementation work, not implied by this projection checkpoint.

## Native AniDB metadata and tracking

Official saved documentation clarified registration, gzip/XML handling, caching, request intervals and non-commercial licensing. One live request using the operator-provided registered client returned a valid anime response; the saved response is parsed locally for compatibility without repeating that dataset request. No source content, user history, password or token is committed.

The adapter loads explicit AniDB identities, normalizes native titles and stable episode IDs, preserves date-only precision, and retains raw source metadata in a server cache. PostgreSQL-backed request reservations survive process restarts, concurrent instances and language changes. Ban/error responses are not catalog data. Native detail pages use AniDB episode identities for ordinary tracking, and SIMKL reviews can separately load their explicit AniDB identity before manual episode resolution.

This checkpoint does not complete title-index search, native artwork/availability, dataset mapping ingestion or alternate-view editing. The official title-index download returned 403. Cached-only search is labelled explicitly, and exact-ID loading remains available. Synthetic cache/history/consent tests and a saved live XML parse are evidence for implemented behavior, not complete browser or deployment acceptance.

## Official native title index

The operator-provided official gzip file verified the title-only XML schema. The application now accepts admin-only XML/gzip index uploads with bounded decompression and identity validation, deduplicates aliases, atomically replaces the index, and records the uploaded checksum and import time. Failure preserves the existing index; repeat identical imports are idempotent. This does not fetch full metadata, change private history, or imply that import time is the source publication time.

Native searches use indexed aliases and the user's requested native/romaji/English title style. An explicit AniDB ID still loads metadata on demand. When no index is installed, cached-only scope remains visibly labelled. The upstream download remains a user/operator step because direct source access returned 403; download at most once per day. No title dataset is bundled in the repository.

## Versioned mapping proposals and corrections

Admin mapping tools now accept checksum-verified animap releases and resolve explicit AniDB-to-TVDB episode identities through TMDB Find By ID. Exact provider episode IDs, not cross-provider numbering, establish the proposed relationship. Inferred defaults and unsupported/missing identities stay in the proposal's unresolved list. An existing local override blocks automatic dataset proposal replacement.

Saving a dataset mapping requires a matching, unexpired server proposal. The server owns dataset provenance and attribution, and rejects altered relationships claiming that provenance. Updates reuse the mapping identity, retain revision history, notify affected users and do not transfer consent to a materially different split/combined graph. Read-only projected flags immediately follow the current revision without modifying source events.

Series overrides now have a dedicated narrow update endpoint, preserve existing statuses/ratings, and survive unrelated library edits. Users can return to their account default. Approved mappings take precedence over unreviewed suggestions; conflicting approved mappings remain blocked. Mapping/source attribution is visible in both review and linked-view screens.

Regression evidence includes the actual pinned release decoder, synthetic differing TVDB/TMDB numbering, a two-native-to-one-target relationship, explicit consent, correction notices, unchanged source history, provenance tampering rejection, local override preservation, revision history, and UI proposal review. Cross-view editing, mapping movies, full Up next aggregation and complete browser acceptance remain further work.

## Linked main tracking and explicit source edits

Main episode tracking now reads effective watched flags from reviewed mappings and revision-bound consent. Marking an already-linked watched episode does not create another history copy or replace its source date. Linked unwatch is a separate explicit choice showing the affected source records. The applied plan is bound to the selected episodes, mapping state and recorded history IDs; changed mappings or replacement source records require a fresh preview.

Unwatch tombstones suppress older/imported counterpart facts. A later genuine local watch action can update the linked view, while imports/restores cannot claim local intent authority. The local action marker is omitted from portable exports and never populated by restore. Mapping changes are locked before account writes to avoid a correction-notice lock inversion. Effective lookups fetch only the relevant mapping neighborhood and source records rather than every private event.

Regression tests cover cancellation, explicit linked confirmation, stale plans, split/combined flags, source-date preservation, no duplicate writes, imported-fact suppression, new local watch intent, portable exclusion and account isolation. Frontend tests flush asynchronous query notifications inside React’s test boundary and bound worker concurrency to two, without relaxing assertions. Movie-link tracking and canonical cross-view statistics remain subsequent work.

## Native artwork and consistent title preferences

Native artwork uses only validated filenames from AniDB's XML response on the verified AniDB image host. Frontend image URLs accept only known source hosts and image paths, reject arbitrary URLs/CSS payloads, and fall back cleanly on failures. A changed image URL retries rather than retaining a previous failure forever. Artwork keeps its original rights; the metadata license is not represented as ownership of the image.

A shared deterministic database presentation function applies native/romaji/English anime title preferences independently of interface and metadata languages. Library, saved lists, smart-list evaluation and cached anime search use the same selection and guarded metadata fallback. Synthetic regressions cover all title styles and malformed image paths; browser visual acceptance remains separate.

### Effective Up next and structure-aware card ordering

Up next now consults the same direct/derived progress and consent rules as tracking. For saved native anime entries with the show structure selected, reviewed links produce cached TMDB show cards using actual provider episode numbering. Cards group by show; no combined rating, status, history record, or library membership is fabricated. An unowned show card opens the native entry's alternate-view screen. Missing, blocked, unknown-release, special, retired, or future counterparts keep an explicitly incomplete native fallback. Native overrides retain native cards. Complete alternate-view editing and broader presentation acceptance remain open.

Private ordering accepts only currently displayed card identities, including derived show cards. Hidden previous ordering is retained, ordinary settings saves preserve it, and portable snapshots validate and carry it. Keyboard-accessible move-up buttons remain alongside drag ordering. `TestUpNextUsesEffectiveProgressAndPrivateStructureOrder` covers source/alternate history, consent, release eligibility, overrides, no invented records, settings retention, and user isolation. Frontend regression coverage checks alternate-view navigation and ordering displayed identities. These are automated backend/jsdom checks, not real-browser acceptance.
