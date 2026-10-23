# Feature checklist

**Status: core APIs and UI are partially implemented. First-release acceptance is not complete. See [validation notes](implementation-notes.md).**

This is the implementation tracker for the [agreed specification](specification.md). A checked box means the behavior is implemented and verified. Keep partial work unchecked; link supporting commits, pull requests, or validation notes when checking an item. Add subitems when a feature needs finer tracking.

The final section is explicitly deferred and does not count toward first-release completion.

## Verification prerequisites

- [ ] Verify current AniDB API access, registration, rate, cache, and attribution requirements.
- [ ] Inspect a private SIMKL JSON backup and document actual import field coverage using redacted examples.
- [ ] Verify the configured OIDC provider supplies current access/admin claims at refresh or UserInfo.
- [ ] Verify mapping coverage for ordinary shows, split cours, specials, and split/combined anime episodes.
- [ ] Pin compatible implementation dependencies and mapping dataset versions.

Checked behaviors below are supported by the named regression tests in [implementation evidence](implementation-notes.md). Browser, live-provider, and anime cross-view acceptance remain separate and unchecked.

## Identity and privacy

- [x] OIDC login with stable issuer/subject identity binding.
- [x] Configurable admission claim name and allowed value.
- [x] Automatic account creation only after admission succeeds.
- [x] Separate configurable admin claim.
- [x] No automatic first-user admin promotion or manual account creation.
- [x] Current admission/admin claim evaluation at login and token refresh.
- [ ] Current claims from validated refreshed ID token or subject-validated UserInfo.
- [x] Access denial when admission claims are absent.
- [x] Admin-right removal when admin claims are absent.
- [x] Require login when current authorization claims cannot be obtained.
- [ ] Server-side authorization configuration recovery without an implicit role bypass.
- [ ] Private per-user libraries, history, ratings, lists, statuses, settings, and import data.
- [ ] Authorization checks on every user-scoped API and admin operation.
- [ ] Admin account oversight without exposing private library browsing.

## Interface and localization

- [ ] Original polished dark, poster-focused interface with compact controls.
- [ ] Responsive desktop layouts.
- [ ] Responsive phone layouts and touch-friendly quick actions.
- [ ] Installable web-app manifest and icons.
- [ ] Persistent main navigation and header search.
- [ ] Home dashboard.
- [ ] Library poster-grid view.
- [x] Library table view.
- [ ] Discovery and global search screens.
- [ ] Movie/show/season/episode detail screens.
- [ ] Person detail screens.
- [ ] Manual-list and smart-list screens.
- [ ] Calendar screens.
- [ ] Statistics screen.
- [ ] User settings and import/export screens.
- [ ] Import preview, conflicts, and review queue screens.
- [ ] Admin mapping and account-oversight screens.
- [ ] Keyboard shortcuts for search, watched actions, and result navigation.
- [ ] Visible equivalents for every keyboard action.
- [ ] Accessible UI primitives, focus handling, and keyboard navigation.
- [ ] Polished loading, empty, error, and retry states.
- [ ] Complete English UI translation.
- [ ] Complete Turkish UI translation.
- [ ] Browser-language initialization with English fallback.
- [ ] Independent interface language, metadata language, anime title style, country, and timezone settings.
- [ ] Metadata language fallback to available source content.

## Catalog and details

- [ ] Movie catalog search and metadata.
- [ ] TV catalog search and metadata.
- [ ] Anime catalog search and native identities.
- [ ] People search.
- [ ] Popular and upcoming browsing.
- [ ] Posters, artwork, synopsis, dates, runtime, and genres.
- [ ] Show ongoing/ended status.
- [ ] Season details and episode lists.
- [ ] Episode titles, summaries, air dates, and available runtime information.
- [ ] Separate specials from regular episode progression.
- [ ] Navigable cast credits.
- [ ] Extended crew browsing.
- [ ] Related titles.
- [ ] On-demand trailers without autoplay.
- [ ] Person biography and image.
- [ ] Person acting and crew filmography across supported content types.
- [ ] Person-credit filters by content type.
- [ ] Person-credit sorting by release date or popularity.

## Discovery

- [ ] Content-type and genre filters.
- [ ] Release year and year-range filters.
- [ ] Cast and crew filters for movie discovery.
- [ ] TV cast/crew discovery through progressively loaded person credits.
- [ ] Honest result counts after local filtering and watched exclusions.
- [ ] Runtime filters.
- [ ] Original production language filter.
- [ ] Community score threshold.
- [ ] Configurable minimum vote count.
- [ ] Country and provider filters.
- [ ] Exclude watched titles without inventing unknown tracking data.
- [ ] Paginated results and progressive enrichment without an exhaustive external catalog mirror.

## Where to watch

- [ ] Per-user country selection.
- [ ] Per-user streaming subscription selection.
- [ ] Optional browsing limited to selected services.
- [ ] Subscription, rental, and purchase categories.
- [ ] Provider or availability-page outbound links.
- [ ] Unknown/unavailable provider states.
- [ ] Availability freshness display.
- [ ] Required TMDB and JustWatch attribution.

## Library statuses and ratings

- [ ] Fixed Plan to watch, Watching, Completed, On hold, and Dropped base statuses.
- [ ] Per-user custom status creation and naming.
- [ ] Custom status mapping to one base status.
- [x] Lock base mapping once a custom status has been used.
- [x] Rename custom statuses without breaking references.
- [x] Require replacement status before deleting a used status.
- [ ] Dashboard and filter behavior based on mapped base status.
- [x] Preserve custom statuses when automatic transitions retain their base.
- [ ] Use the default destination status when automatic transitions change base.
- [x] Personal whole-number ratings from 1 to 10 for movies and whole shows.
- [ ] Anime-native entry ratings and statuses.
- [ ] Personal library searching, sorting, and filtering.

## Viewing history and progress

- [x] Mark individual episodes watched/unwatched.
- [x] Mark entire seasons watched/unwatched.
- [x] Mark entire shows watched/unwatched.
- [ ] Mark movies watched/unwatched.
- [ ] Today's date, chosen date, and unknown date for bulk watch actions.
- [x] Preserve existing known watched dates during bulk actions.
- [x] Mark only the selected episode by default.
- [x] Detect earlier unwatched aired regular episodes.
- [x] Prompt for this episode only or earlier episodes too, across prior seasons and excluding specials.
- [x] Exclude unaired episodes from missed-episode counts.
- [x] Keep caught-up ongoing shows Watching.
- [ ] Complete ended shows after all regular episodes are watched, with manual overrides.
- [x] Offer Watching after unwatching a completed show's episode, preserving status unless accepted.
- [ ] Preserve watch events through metadata ID, ordering, and count changes.
- [ ] Flag uncertain episode reconciliation instead of silently moving or deleting history.

## Dashboard, calendar, spoilers, and notices

- [ ] Up next uses the earliest unwatched aired regular episode of actively watched shows.
- [ ] Up next excludes unaired episodes and separates specials.
- [ ] Manual Up next ordering.
- [ ] Recent activity on the dashboard.
- [ ] Upcoming episodes on the dashboard.
- [ ] Calendar month view.
- [ ] Calendar list view.
- [ ] Preserve date-only release precision.
- [ ] Display known release timestamps in each user's timezone.
- [ ] Per-user spoiler preference hiding unwatched episode summaries and images.
- [ ] Explicit spoiler reveal controls.
- [ ] In-app notices.

## Manual and smart lists

- [ ] Private named manual lists.
- [ ] Manual-list drag ordering.
- [ ] Smart lists over personal-library titles only.
- [ ] Visual nested AND/OR rule groups.
- [ ] NOT exclusions.
- [ ] Live matching count and title preview before saving.
- [ ] Content type, exact/base status, genre, year, and personal-rating conditions.
- [ ] Country availability and streaming-service conditions.
- [ ] Explicit unknown/unset conditions.
- [x] Negative filters do not silently match unknown values.
- [ ] Manual-list membership conditions.
- [ ] Smart-list membership conditions.
- [ ] Stable list identifiers for dependencies.
- [x] Cycle prevention across smart-list dependencies.
- [x] Show dependent lists and block deletion until references are resolved.
- [x] Safe list renaming.
- [ ] Saved smart-list sorting by title, release date, rating, or date added.

## Anime structures and mappings

- [ ] Native anime entry/cour organization and numbering.
- [ ] TMDB-style show/season organization.
- [ ] Per-user default structure.
- [ ] Per-series structure override.
- [ ] One shared underlying episode history across views.
- [ ] Stable source identities without merging entries solely on shared TMDB IDs.
- [ ] Mapping provenance and dataset versions.
- [ ] Native-entry statuses and ratings visible in aggregate show views.
- [ ] Aggregate progress without fabricated combined ratings.
- [ ] Up next cards follow the selected structure.
- [ ] Missing/ambiguous mappings retain usable source entries.
- [ ] Incomplete alternate-view indicators.
- [ ] Admin mapping review and local override tools.
- [ ] Explicit versus inferred mapping distinction.
- [ ] Per-user confirmation before split/combined cross-view progress linking.
- [ ] Explain connected episodes during mapping confirmation.
- [x] Prevent unconfirmed mapping propagation.
- [ ] Preserve recorded events when mappings change.
- [ ] Recompute derived progress after corrections.
- [ ] Notify affected users about mapping corrections.
- [x] Renew confirmation for materially changed split/combined relationships.
- [ ] Avoid double-counting history across alternate views.

## SIMKL migration

Current support is partial: explicit TMDB movie/show IDs can be loaded on demand, and supported records/history can be applied after confirmation; source episode mappings and uncached/native identities require review. The private queue now supports explicit one-to-one episode resolution and keeping existing app edits; split/combined identities and malformed source records remain unresolved. Custom lists are archived, not imported. See `implementation-notes.md`.

- [x] JSON-backup ingestion and validation.
- [ ] Read-only API import when verified JSON coverage is insufficient.
- [ ] Import source identifiers without unsafe anime merges.
- [ ] Import statuses and episode progress.
- [ ] Import recorded watched dates and personal ratings.
- [ ] Represent inferred completion dates as unknown.
- [ ] Import custom-list membership.
- [ ] Translate source smart rules only when exact.
- [x] Preserve untranslated rules and unsupported fields in the source archive.
- [x] Preserve imported SIMKL snapshots.
- [ ] Preview additions, changes, conflicts, unresolved matches, and errors.
- [x] Require user application after preview.
- [ ] Apply resolved items without blocking on ambiguous items.
- [x] Persistent unresolved-item review queue.
- [ ] Existing app edits win on repeat imports.
- [ ] Review conflicts and add missing history without deleting app records.
- [ ] Idempotent repeat import without duplicated records.
- [x] Visible backup/export prerequisite before import, with no import-specific undo.
- [ ] Manual reimport without ongoing SIMKL sync.

## Portability and recovery

- [ ] Per-user portable UI export.
- [ ] Export library, history/dates, ratings, custom statuses, lists/rules, and settings.
- [ ] Per-user UI restore with validation and preview.
- [ ] Restore conflict handling consistent with migration.
- [ ] Exclude authorization roles, server secrets, and sessions from library restore authority.
- [ ] Document infrastructure database backup and restoration.
- [x] Validate a database recovery workflow with synthetic data.

## Statistics

- [ ] Watched movie counts.
- [ ] Watched episode counts.
- [ ] Estimated watched runtime.
- [ ] Monthly activity.
- [ ] Genre breakdowns.
- [ ] Exclude unknown watched dates from date-based charts.

## Metadata and background operation

- [ ] TMDB metadata adapter.
- [ ] TMDB/JustWatch availability adapter.
- [ ] AniDB integration after access-policy verification.
- [ ] Versioned animap ingestion with applicable attribution and license handling.
- [ ] On-demand metadata fetching and PostgreSQL caching.
- [ ] Daily refresh of active shows.
- [ ] Weekly refresh of other saved titles.
- [ ] Daily availability refresh for recently used countries.
- [ ] Manual refresh.
- [ ] Visible metadata freshness.
- [ ] Retryable background jobs and visible failures.
- [ ] Cached library metadata and tracking during provider outages.
- [ ] Retryable errors for uncached search/details.
- [ ] Graceful missing external artwork behavior.

## Architecture and packaging

- [ ] Go standard-HTTP API backend.
- [x] PostgreSQL schema and migrations.
- [x] `pgx` and `sqlc` database access.
- [ ] React/TypeScript/Vite frontend.
- [ ] Tailwind and accessible UI primitives.
- [ ] TanStack Query API state and mutations.
- [ ] Background jobs in the app process with PostgreSQL state.
- [ ] App container serving frontend and API.
- [ ] Docker Compose deployment.
- [ ] Helm chart deployment.
- [x] External PostgreSQL connection configuration.
- [x] OIDC/provider secret configuration without committed credentials.
- [ ] Document build, lint, test, configuration, and deployment commands.

## Acceptance and quality

- [ ] Backend tests for claims, user isolation, and admin operations.
- [ ] Tests for tracking, date precision, bulk actions, and status transitions.
- [x] Tests for custom-status locking and safe deletion.
- [x] Tests for nested smart rules, unknown values, dependencies, and cycle rejection.
- [x] Tests for mapping consent, corrections, and history preservation.
- [x] Tests for import preview, idempotency, conflicts, and unresolved items.
- [x] Tests for export/restore and user isolation.
- [ ] Browser checks for search, details, providers, and library workflows.
- [ ] Browser checks for tracking and smart-list workflows.
- [ ] English and Turkish layout review.
- [ ] Desktop and phone visual review.
- [ ] Keyboard and focus accessibility review.
- [ ] Compose deployment smoke check.
- [ ] Helm deployment smoke check.
- [ ] Provider-outage behavior check.
- [ ] Synthetic/redacted committed fixtures only.

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
