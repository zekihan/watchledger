# Feature checklist

**Status: implemented workflows are under validation; first-release acceptance is not complete. See [validation notes](implementation-notes.md) and the [operator handoff](../TODO.md).**

This is the implementation tracker for the [agreed specification](specification.md). A checked box means the behavior is implemented and verified. Keep partial work unchecked; link supporting commits, pull requests, or validation notes when checking an item. Add subitems when a feature needs finer tracking.

Unchecked items include source coverage, configured-provider, browser/device and deployment acceptance; they do not all mean absent code. Operator-only prerequisites and decisions are collected in TODO.md and are not waived by that handoff.

The final section is explicitly deferred and does not count toward first-release completion.

## Verification prerequisites

- [x] Verify current AniDB API access, registration, rate, cache, and attribution requirements.
- [x] Inspect a private SIMKL JSON backup and document actual import field coverage using [synthetic examples and validation limits](acceptance-next-steps.md#observed-simkl-backup-coverage).
- [x] Verify the configured OIDC provider supplies current access/admin claims at refresh or UserInfo: real Authelia admin login and forced refresh passed on 2026-10-24; operator-deferred non-admin/revocation cases remain separate.
- [x] Verify mapping coverage for ordinary shows, split cours, specials, and split/combined anime episodes. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Pin compatible implementation dependencies and mapping dataset versions.

Checked behaviors below are supported by the named regression tests in [implementation evidence](implementation-notes.md). Actual Chromium coverage is recorded in the independent audit. Real-provider, physical-device and broad anime coverage remain separate wherever explicitly open.

## Identity and privacy

- [x] OIDC login with stable issuer/subject identity binding.
- [x] Configurable admission claim name and allowed value.
- [x] Automatic account creation only after admission succeeds.
- [x] Separate configurable admin claim.
- [x] No automatic first-user admin promotion or manual account creation.
- [x] Current admission/admin claim evaluation at login and token refresh.
- [x] Current claims from validated refreshed ID token or subject-validated UserInfo.
- [x] Access denial when admission claims are absent.
- [x] Admin-right removal when admin claims are absent.
- [x] Require login when current authorization claims cannot be obtained.
- [x] Server-side authorization configuration recovery without an implicit role bypass. Verified in the [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Private per-user libraries, history, ratings, lists, statuses, settings, and import data.
- [x] Authorization checks on every user-scoped API and admin operation.
- [x] Admin account oversight without exposing private library browsing.

## Interface and localization

- [x] Original polished dark, poster-focused interface with compact controls. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Responsive desktop layouts. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [ ] Responsive phone layouts and touch-friendly quick actions.
  - [x] Main English/Turkish screens fit verified Chrome phone viewports; mapping-panel overflow fixed. Physical Android Chrome acceptance remains deferred.
- [ ] Installable web-app manifest and icons.
  - [x] Dimension-verified raster manifest/touch icons and explicit browser-provided install flow.
  - [ ] Target-device installation and authentication-return checks.
- [x] Persistent main navigation and header search. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Home dashboard. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Library poster-grid view. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Library table view.
- [x] Discovery and global search screens. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Movie/show/season/episode detail screens.
  - [x] Source-ID episode deep links, season detail presentation, spoiler-safe episode stills and missing-identity handling.
  - [x] Movie/TV/native episode presentation verified in the Chromium fixture matrix; representative live-source checks remain in the dated acceptance record.
- [x] Person detail screens. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Manual-list and smart-list screens. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Calendar screens. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Statistics screen. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] User settings and import/export screens. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Import preview, conflicts, and review queue screens. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Admin mapping and account-oversight screens. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Keyboard shortcuts for search, watched actions, and result navigation. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Visible equivalents for every keyboard action. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [ ] Accessible UI primitives, focus handling, and keyboard navigation.
- [ ] Polished loading, empty, error, and retry states.
- [x] Complete English UI translation. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Complete Turkish UI translation. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Browser-language initialization with English fallback. See [independent audit evidence](remaining-feature-audit.md).
- [x] Independent interface language, metadata language, anime title style, country, and timezone settings. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Metadata language fallback to available source content. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).

## Catalog and details

- [x] Movie catalog search and metadata. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] TV catalog search and metadata. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [ ] Anime catalog search and native identities.
  - [x] Explicit AniDB ID lookup and honestly labelled cached-alias search.
  - [x] Search aliases from an admin-uploaded official title-only index with atomic replacement.
  - [ ] Complete native presentation acceptance.
- [x] People search. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Popular and upcoming browsing. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Posters, artwork, synopsis, dates, runtime, and genres. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Show ongoing/ended status. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Season details and episode lists. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Episode titles, summaries, air dates, and available runtime information. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Separate specials from regular episode progression. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Navigable cast credits. See [independent audit evidence](remaining-feature-audit.md).
- [x] Extended crew browsing. See [independent audit evidence](remaining-feature-audit.md).
- [x] Related titles. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] On-demand trailers without autoplay. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Person biography and image. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Person acting and crew filmography across supported content types. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Person-credit filters by content type. See [independent audit evidence](remaining-feature-audit.md).
- [x] Person-credit sorting by release date or popularity. See [independent audit evidence](remaining-feature-audit.md).

## Discovery

- [x] Content-type and genre filters. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Release year and year-range filters. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Cast and crew filters for movie discovery. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] TV cast/crew discovery through progressively loaded person credits. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Honest page-local result counts after filtering and watched exclusions, including unavailable-library handling.
- [x] Runtime filters. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Original production language filter. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Community score threshold. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Configurable minimum vote count. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Country and provider filters. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Exclude watched titles without inventing unknown tracking data. See [independent audit evidence](remaining-feature-audit.md).
- [x] Paginated results and progressive enrichment without an exhaustive external catalog mirror. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).

## Where to watch

- [x] Per-user country selection. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Per-user streaming subscription selection. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Optional browsing limited to selected services. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Subscription, rental, and purchase categories. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Provider or availability-page outbound links. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Unknown/unavailable provider states. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Availability freshness display. Verified in the [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Required TMDB and JustWatch attribution. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).

## Library statuses and ratings

- [x] Fixed Plan to watch, Watching, Completed, On hold, and Dropped base statuses.
- [x] Per-user custom status creation and naming.
- [x] Custom status mapping to one base status.
- [x] Lock base mapping once a custom status has been used.
- [x] Rename custom statuses without breaking references.
- [x] Require replacement status before deleting a used status.
- [x] Dashboard and filter behavior based on mapped base status. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Preserve custom statuses when automatic transitions retain their base.
- [x] Use the default destination status when automatic transitions change base. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Personal whole-number ratings from 1 to 10 for movies and whole shows.
- [x] Anime-native entry ratings and statuses. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Personal library searching, sorting, and filtering. See [independent audit evidence](remaining-feature-audit.md).

## Viewing history and progress

- [x] Mark individual episodes watched/unwatched.
- [x] Mark entire seasons watched/unwatched.
- [x] Mark entire shows watched/unwatched.
- [x] Mark movies watched/unwatched. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Today's date, chosen date, and unknown date for bulk watch actions. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Preserve existing known watched dates during bulk actions.
- [x] Mark only the selected episode by default.
- [x] Detect earlier unwatched aired regular episodes.
- [x] Prompt for this episode only or earlier episodes too, across prior seasons and excluding specials.
- [x] Exclude unaired episodes from missed-episode counts.
- [x] Keep caught-up ongoing shows Watching.
- [x] Complete ended shows after all regular episodes are watched, with manual overrides. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Offer Watching after unwatching a completed show's episode, preserving status unless accepted.
- [x] Preserve watch events through metadata ID, ordering, and count changes. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Flag uncertain episode reconciliation instead of silently moving or deleting history. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).

## Dashboard, calendar, spoilers, and notices

- [x] Up next uses the earliest unwatched aired regular episode of actively watched shows. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Up next excludes unaired episodes and separates specials. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Manual Up next ordering. See [independent audit evidence](remaining-feature-audit.md).
- [x] Recent activity on the dashboard. See [independent audit evidence](remaining-feature-audit.md).
- [x] Upcoming episodes on the dashboard. See [independent audit evidence](remaining-feature-audit.md).
- [x] Calendar month view. Readable desktop grid and phone-selectable horizontal grid; phones default to agenda. See [audit](remaining-feature-audit.md).
  - [x] Date-only preservation, timestamp timezone placement, source episode links and day-list semantics in UI regression tests.
  - [x] Desktop/phone-sized Chromium visual review and automated accessibility scans.
  - [ ] Intended physical-device and screen-reader acceptance.
- [x] Calendar list view. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Preserve date-only release precision. See [independent audit evidence](remaining-feature-audit.md).
- [x] Display known release timestamps in each user's timezone. See [independent audit evidence](remaining-feature-audit.md).
- [x] Per-user spoiler preference hiding unwatched episode summaries and images. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Explicit spoiler reveal controls. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Private in-app notices with explicit, idempotent acknowledgement and retryable errors.

## Manual and smart lists

- [x] Private named manual lists.
- [x] Manual-list drag ordering, visible keyboard/touch move controls, and localized position announcements. Verified by `web/src/lists.test.tsx`; physical-device acceptance remains separate.
- [x] Smart lists over personal-library titles only.
- [x] Visual nested AND/OR rule groups. See [independent audit evidence](remaining-feature-audit.md).
- [x] NOT exclusions. See [independent audit evidence](remaining-feature-audit.md).
- [x] Live matching count and title preview before saving. See [independent audit evidence](remaining-feature-audit.md).
- [x] Content type, exact/base status, genre, year, and personal-rating conditions. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Country availability and streaming-service conditions. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Explicit unknown/unset conditions. See [independent audit evidence](remaining-feature-audit.md).
- [x] Negative filters do not silently match unknown values.
- [x] Manual-list membership conditions. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Smart-list membership conditions. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Stable list identifiers for dependencies.
- [x] Cycle prevention across smart-list dependencies.
- [x] Show dependent lists and block deletion until references are resolved.
- [x] Safe list renaming.
- [x] Saved smart-list sorting by title, release date, rating, or date added.

## Anime structures and mappings

- [ ] Native anime entry/cour organization and numbering.
  - [x] Source episode types/numbers, native credits/related titles and bounded official Hot Anime discovery.
  - [ ] Broader source coverage and presentation acceptance.
- [x] TMDB-style show/season organization. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Per-user default structure. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Per-series structure override.
- [ ] One shared underlying episode history across views.
  - [x] Confirmed episode-level alternate tracking without duplicate library entries, including native status updates and portable source history.
  - [ ] Full mixed-relationship and cross-view/browser acceptance.
- [x] Stable source identities without merging entries solely on shared TMDB IDs. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Mapping provenance and dataset versions.
- [x] Native-entry statuses and ratings visible in aggregate show views.
- [x] Aggregate progress without fabricated combined ratings. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [ ] Up next cards follow the selected structure.
  - [x] Effective-progress native-to-show cards, incomplete source fallback, and private displayed-card ordering.
  - [x] Synthetic combined alternate-view tracking, consent cancellation/confirmation and linked-unwatch browser acceptance in all three engines. Broader real-source/device coverage remains separate.
- [x] Missing/ambiguous mappings retain usable source entries. See the Frieren correction exercise in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Incomplete alternate-view indicators. See the Frieren correction exercise in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [ ] Admin mapping review and local override tools.
  - [x] Versioned episode proposals, local override protection, revision history and provenance validation.
  - [x] Explicit movie-alias proposals, local overrides, distinct consent and source-safe tracking.
  - [ ] Full browser and broad mapping-coverage acceptance.
- [x] Explicit versus inferred mapping distinction.
- [x] Per-user confirmation before split/combined cross-view progress linking. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Explain connected episodes during mapping confirmation.
- [x] Prevent unconfirmed mapping propagation.
- [x] Preserve recorded events when mappings change. See the Frieren correction exercise in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Recompute derived progress after corrections. See the Frieren correction exercise in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Notify affected users about mapping corrections.
- [x] Renew confirmation for materially changed split/combined relationships.
- [ ] Avoid double-counting history across alternate views.
  - [x] Confirmed episode mappings use view-independent native units, separate partial progress, and labelled date/ambiguity handling.
  - [ ] Broad mixed-relationship and final browser acceptance.

## SIMKL migration

Current source resolution remains conservative: explicit TMDB movie/show IDs can be loaded on demand, and supported records/history can be applied after confirmation; source episode mappings and uncached/native identities require review. The private queue now supports explicit one-to-one episode resolution and keeping existing app edits; split/combined identities and malformed source records remain unresolved. A separate documented custom-list API-response file can be imported as a manual snapshot when all members resolve; the inspected ordinary backup has no custom-list section, and representative live API acceptance is recorded in `acceptance-2026-10-24.md`. Remaining account/source edge cases stay distinct. See `implementation-notes.md`.

- [x] JSON-backup ingestion and validation.
- [ ] Read-only API import when verified JSON coverage is insufficient.
  - [x] Optional GET-only custom-list fetch with ephemeral credentials, complete pagination, private caching and preview/apply safeguards.
  - [ ] Live AUTH V2 and source-account coverage acceptance.
- [x] Import source identifiers without unsafe anime merges. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Import statuses and episode progress. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Import recorded watched dates and personal ratings. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Represent inferred completion dates as unknown. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [ ] Import custom-list membership.
  - [x] Import complete documented API-response files as ordered manual snapshots, with private identity resolution and app edit/deletion preservation.
  - [ ] Verify ordinary backup custom-list coverage and live source acceptance.
- [ ] Translate source smart rules only when exact.
- [x] Preserve untranslated rules and unsupported fields in the source archive.
- [x] Preserve imported SIMKL snapshots.
- [x] Preview additions, changes, conflicts, unresolved matches, and errors. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Require user application after preview.
  - [x] Cancel superseded and abandoned preview requests across SIMKL backups, custom-list response files/live fetches, and portable restores. Delayed file reads cannot start uploads after navigation; late responses cannot restore discarded previews. Verified with synthetic UI regressions.
- [x] Apply resolved items without blocking on ambiguous items. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Persistent unresolved-item review queue.
- [x] Existing app edits win on repeat imports. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Review conflicts and add missing history without deleting app records. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Idempotent repeat import without duplicated records. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Visible backup/export prerequisite before import, with no import-specific undo.
- [x] Manual reimport without ongoing SIMKL sync. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).

- [x] Prepare explicit source identities and seasons in bounded, resumable batches; continue a saved preview without re-uploading it.
- [x] Inspect and batch-confirm regular TMDB episode proposals, with version checks and isolated exception review; preserve gaps, unknown dates and existing user decisions.
- [x] Download the prerequisite export inline before applying and focus remaining review items afterward. Synthetic regressions: `internal/httpapi/simkl_prepare_test.go` and `web/src/imports.test.tsx`; real-provider coverage remains separate.

## Portability and recovery

- [x] Per-user portable UI export. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Export library, history/dates, ratings, custom statuses, lists/rules, and settings. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Per-user UI restore with validation and preview. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Restore conflict handling consistent with migration. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Exclude authorization roles, server secrets, and sessions from library restore authority. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Document infrastructure database backup and restoration. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Validate a database recovery workflow with synthetic data.

## Statistics

Backend regressions verify private counts, runtime unknowns, metadata-language selection and timezone/date precision. Chrome additionally verified native-unit totals/runtime, partial/source-part counts, visible genre/monthly charts and unknown-date exclusion; this is representative synthetic-data acceptance.

- [x] Watched movie counts. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Watched episode counts. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Estimated watched runtime. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Monthly activity. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Genre breakdowns. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Exclude unknown watched dates from date-based charts. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).

## Metadata and background operation

- [x] TMDB metadata adapter. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] TMDB/JustWatch availability adapter. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] AniDB integration after access-policy verification. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md).
  - [x] On-demand native metadata/episode loading, registered client configuration, persistent cooldowns, source-ID-safe tracking and cached outage fallback.
  - [x] Official title-index file import and native title search.
  - [x] Native artwork URL validation and failed-artwork fallback.
  - [ ] Complete native/alternate-view browser acceptance.
- [x] Versioned animap ingestion with applicable attribution and license handling.
- [x] On-demand metadata fetching and PostgreSQL caching. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md). Default-language detail fallbacks must match the requested source identity before merging or caching; `internal/catalog/fallback_identity_test.go` covers invalid IDs and preservation of existing default metadata with synthetic HTTP fixtures and PostgreSQL.
- [ ] Daily refresh of active shows.
  - [x] Queue saved TMDB and native AniDB metadata with provider-aware retry/cooldown handling.
  - [ ] Live deployment refresh acceptance.
- [x] Weekly refresh of other saved titles. See [independent audit evidence](remaining-feature-audit.md).
- [x] Daily availability refresh for recently used countries. See [independent audit evidence](remaining-feature-audit.md).
- [x] Manual refresh. Verified in the [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Visible metadata freshness. Verified in the [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Retryable background jobs and visible failures. Verified in the [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Cached library metadata and tracking during provider outages. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Retryable errors for uncached search/details. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Graceful missing external artwork behavior. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).

## Architecture and packaging

- [x] Go standard-HTTP API backend.
- [x] PostgreSQL schema and migrations.
- [x] `pgx` and `sqlc` database access.
- [x] React/TypeScript/Vite frontend.
- [ ] Tailwind and accessible UI primitives.
- [x] TanStack Query API state and mutations.
- [x] Background jobs in the app process with PostgreSQL state.
- [x] App container serving frontend and API.
- [ ] Docker Compose deployment.
  - [x] Synthetic runtime startup, OIDC login, restore/tracking/export and restart persistence in CI.
  - [ ] Operator configuration/environment acceptance.
- [x] Helm chart deployment. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] External PostgreSQL connection configuration.
- [x] OIDC/provider secret configuration without committed credentials.
- [x] Document build, lint, test, configuration, and deployment commands.

## Acceptance and quality

The 2026-10-26 regression pass adds request-draining coverage in `cmd/watchledger/main_test.go`; source-current completion, alternate episode ordering, localized season reconciliation and manual refresh coverage in `internal/httpapi/tracking_refresh_regression_test.go`; and country-rule, repeat-import and Turkish status-name coverage in `internal/httpapi/private_library_regression_test.go`. `web/src/metadata-refresh.test.tsx` checks refreshed episode rendering and whole-show unwatch during an outage using Vitest/jsdom; it does not establish real-browser or device acceptance.

- [x] Backend tests for claims, user isolation, and admin operations.
- [x] Tests for tracking, date precision, bulk actions, and status transitions.
- [x] Tests for custom-status locking and safe deletion.
- [x] Tests for nested smart rules, unknown values, dependencies, and cycle rejection.
- [x] Tests for mapping consent, corrections, and history preservation.
- [x] Tests for import preview, idempotency, conflicts, and unresolved items.
- [x] Tests for export/restore and user isolation.
- [x] Browser checks for search, details, providers, and library workflows. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] Browser checks for tracking and smart-list workflows. See [family evidence and acceptance boundaries](remaining-feature-audit.md#contract-reconciliation-by-feature-family).
- [x] English and Turkish layout review. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Desktop and phone visual review. See [browser, localization and acceptance evidence](remaining-feature-audit.md).
- [x] Automated keyboard, dialog cancellation and focus-return review in Chromium, Firefox and WebKit. Intended physical-device/screen-reader acceptance remains separate in TODO.md.
- [x] Synthetic Compose deployment smoke check; actual provider configuration remains in TODO.md.
- [x] Helm deployment smoke check. See the bounded local/provider evidence in [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Provider-outage behavior check. Verified in the bounded [2026-10-24 acceptance](acceptance-2026-10-24.md).
- [x] Synthetic/redacted committed fixtures only.

## Deferred beyond the first release

These remain unimplemented and are not required to complete the first release. Promote them into scope only through an explicit specification change.

- [ ] Automatic Plex tracking.
- [ ] Automatic Jellyfin tracking.
- [ ] Streaming-site tracking and browser extensions.
- [ ] Native mobile applications.
- [ ] Offline editing.
- [ ] Social features, shared lists, and public profiles.
- [ ] Invitations and open registration.
- [ ] Custom recommendation engine.
- [ ] Subscription billing and payment processing.
- [ ] Email, push, and external notification delivery.
- [ ] External iCal subscription.
- [ ] Separate rewatch sessions.
- [ ] Season and episode personal ratings.
- [ ] Guaranteed direct playback links.
- [ ] Episode-level streaming availability.
- [ ] Verified dub/subtitle availability.
- [ ] Ongoing or bidirectional SIMKL synchronization.
- [ ] Exhaustive locally indexed external catalog.
