# Application specification

Agreed on 2026-10-23. Status: approved design; implementation in progress.

This is the first-release product contract. The [feature checklist](features.md) records delivery status. Checked items must represent implemented and verified behavior, not agreement with this document.

## 1. Purpose and boundaries

Build a personal, self-hosted movie, TV, and anime tracker that replaces SIMKL as the primary home for viewing history. Support multiple users with separate private libraries.

The first release must be useful for daily tracking and visually polished. It is an original application inspired by the tracking workflow, not a pixel-for-pixel SIMKL clone.

The responsive web application must work on desktop and phone and be installable to a home screen. Native applications and offline editing are deferred. Provider outages must not prevent tracking against cached library data; this does not imply complete offline application support.

## 2. Accounts, authorization, and privacy

- Use OIDC as the only login method; no local passwords or manual account creation.
- Configure an admission claim name and allowed value. Create a user automatically only after successful authentication and admission checks.
- Configure a separate admin claim. Being first to log in does not grant admin rights.
- Bind identities to the provider's stable issuer and subject, not to mutable display names or email alone.
- Reevaluate admission and admin rules at login and token refresh using current validated claims. Do not silently reuse old authorization claims.
- Missing admission claims deny access. Missing admin claims remove admin rights.
- If current authorization claims cannot be obtained, require login again rather than extending old permissions. Validate the subject of UserInfo responses when that endpoint supplies claims.
- Revocation becomes effective when fresh authorization is evaluated; immediate provider-side revocation delivery is not promised.
- Do not implement manual in-app role promotions or automatic first-user bootstrap.
- Recover authorization configuration through server-side administration; recovery must not silently bypass the claim policy.
- Keep every user's history, ratings, statuses, lists, settings, and import material private from other users. Admin catalog tools must not turn private libraries into a shared browsing surface.
- Shared catalog mappings are controlled by admins.

The actual identity provider, client registration, claim paths/values, and refresh behavior are deployment configuration, not assumptions about an existing installation.

## 3. Interface and navigation

Ship a polished dark interface with poster-focused browsing and compact controls. Support both desktop and phone layouts. Provide complete English and Turkish UI translations in the first release.

Required surfaces:

- Home dashboard with Up next, recent activity, and upcoming episodes.
- Personal library with poster-grid and table views.
- Discovery, global search, and search results.
- Movie, show, season, episode, and person details.
- Manual lists and smart-list builder/results.
- Month and list calendar views.
- Statistics.
- User settings and portable import/export.
- Import preview, conflict reporting, and unresolved-item review queue.
- Admin account oversight and shared mapping review/correction.

Use persistent main navigation, header search, and visible quick tracking controls on desktop and phone. Add keyboard shortcuts for search, marking watched, and navigating results without making shortcuts the only way to act.

Interface language, metadata language, preferred anime title style, country, and timezone are independent user settings. Browser language initializes supported UI language; unavailable UI translations fall back to English. Metadata falls back to the source's default when the requested translation is missing.

## 4. Catalog and detail pages

Support movies, TV shows, and anime from the first release. Search titles and people, and browse popular, upcoming, related, and filtered results.

### Movie and show details

Include poster/artwork, synopsis, dates, runtime, genres, cast, extended crew, related titles, and provider links. Trailers play only on request; no automatic video playback.

TV and anime details include seasons or native entries as appropriate, episode lists, episode summaries, air dates, and ongoing/ended status. Make cast and crew entries navigable to person pages.

### Person pages

Provide biography, image, acting credits, crew credits, and movie/TV/anime coverage where data exists. Support content-type filtering and sorting by release date or popularity.

### Discovery filters

Include content type, genre, year/range, cast/crew, runtime, original production language, community score, minimum vote count, country availability, provider, and watched-title exclusion.

Community score is distinct from the user's personal rating. The minimum vote count is configurable. Language means original production language; available dubs and subtitles are not included.

Use provider-supported catalog discovery instead of importing the entire external catalog. TMDB movie discovery supports cast/crew filters; TV cast/crew discovery uses person credits as the candidate set, with progressive filtering/enrichment. Do not describe this as exhaustive global TV discovery or show unfiltered totals as though they are exact after local exclusions.

Availability and missing history must not be fabricated to make a result satisfy a filter.

## 5. Where to watch

Each user chooses a country and a set of streaming subscriptions. Provide optional filtering to their selected services.

Distinguish subscription, rental, and purchase offers. Link to the provider or supplied availability page. Guaranteed direct playback links and episode-level availability are outside the first-release contract.

Show unavailable or unknown provider data honestly and surface freshness. Apply required source attribution.

## 6. Library statuses and ratings

The immutable base statuses are:

- Plan to watch.
- Watching.
- Completed.
- On hold.
- Dropped.

Each user can create custom statuses mapped to exactly one base status. Dashboards, filters, watched-related behavior, and progress transitions use that mapping. Lock the mapping once the custom status has been used. Renaming must not change identity or base behavior. Deletion requires moving affected titles to a replacement status.

When progress triggers a status transition, preserve a custom status if it maps to the appropriate destination base. Otherwise move the title to that base's default status.

Allow whole-number personal ratings from 1 through 10 on movies and whole shows. For anime, the native entry owns its rating as specified below. Episode and season ratings are deferred.

## 7. Viewing history and episode progress

Support marking individual episodes, entire seasons, entire shows, and movies watched or unwatched. Bulk watch actions offer today's date, a chosen date, or unknown date. Do not overwrite existing known dates merely because a bulk operation touches them.

Marking an episode watched affects only that episode by default. If earlier aired regular episodes remain unwatched, offer:

- Only this episode, selected by default.
- Also mark earlier aired regular episodes, including earlier seasons and excluding specials.

Keep specials separate from ordinary progression and do not count unaired episodes as missed.

Caught-up ongoing shows remain Watching. An ended show becomes Completed once its regular episodes are all watched, with manual overrides available. If an episode is marked unwatched on a completed show, offer to return the show to Watching; retain the current status unless the user accepts.

Preserve watch events when upstream episode IDs, order, counts, or mappings change. Flag uncertain reconciliation rather than deleting history or silently moving it to a different episode.

Separate rewatch sessions are not part of the first release. Unsupported imported rewatch fields remain in the source archive.

## 8. Home dashboard, calendar, spoilers, and statistics

### Up next

For each actively watched show, display the earliest unwatched aired regular episode. Exclude unaired episodes and keep specials separate. Support manual ordering. Anime cards follow the user's selected native versus show/season structure while sharing one underlying history.

### Calendar

Include a dedicated month/list calendar in addition to dashboard upcoming episodes. Preserve date-only releases as date-only values. Convert known release timestamps into the user's timezone rather than inventing midnight timestamps for imprecise data.

### Spoilers and notices

Hide unwatched episode summaries and images by default through a per-user spoiler setting. Allow explicit reveal. Provide in-app notices; external email, push, and notification-service integrations are deferred.

### Statistics

Include watched movie counts, watched episode counts, estimated runtime, monthly activity, and genre breakdowns. Keep unknown watched dates outside date-based charts. Alternate anime views must not double-count the same underlying history.

Counting clarification approved on 2026-10-23: confirmed anime episode mappings use native AniDB episodes as the counting units regardless of the selected view. Show partially watched native episodes separately rather than adding them to completed-episode totals. Unconfirmed or ambiguous relationships must be labelled as unresolved rather than silently deduplicated.

## 9. Manual and smart lists

Manual lists are private, named collections with drag ordering. Smart lists evaluate only titles in the user's personal library and have a saved sort such as title, release date, personal rating, or date added.

The visual smart-list builder supports nested AND/OR groups and NOT exclusions, with a live result count and preview before saving.

Supported conditions include content type, exact status, base status, genres, release year, personal rating, country availability, selected services, and membership in manual or smart lists.

Reference lists through stable identifiers. Reject cycles in smart-list dependencies. Renaming a referenced list is safe; deleting it is blocked until its references are removed or replaced, with dependents shown to the user.

Treat unknown/unset data explicitly. An unknown value must not silently satisfy a negative condition. For example, unknown availability does not match "not available on Netflix" unless the rule explicitly includes unknown availability.

## 10. Anime identity and alternate structures

Support both native anime entries, including separate cours and native numbering, and TMDB-style show/season organization. Provide a per-user default and a per-series override.

Both presentations are views over one shared episode history, not independent tracking copies. Preserve stable source identities and mapping provenance rather than treating a shared TMDB show ID as proof that two native entries are identical.

Native entries own their statuses and personal ratings. A whole-show view aggregates progress and presents constituent entries' statuses/ratings without inventing a combined rating.

Up next presents native-entry cards in native mode and show cards in show/season mode.

Missing or ambiguous mappings must leave source entries usable. Flag the alternate view as incomplete and provide admin mapping review/correction tools. Prefer explicit mappings over unreviewed inferred matches.

For split/combined mappings, each user confirms the mapping once before cross-view progress is linked. Until confirmation, do not silently propagate watched flags across that relationship. The mapping review must explain which episodes are connected.

Mapping corrections preserve recorded events, recompute derived alternate-view progress, and notify affected users for review. A materially changed split/combined mapping must not inherit consent to a different relationship silently.

Proposed sources: AniDB-native identity/metadata, TMDB, and animap releases, preserving imported SIMKL snapshots. Current AniDB API access and usage requirements must be verified before implementing the integration. If unsuitable, review a Kitsu fallback rather than silently changing entry semantics. See [data sources](data-sources.md).

## 11. SIMKL migration

SIMKL is a one-time migration source with manual repeat imports, not an ongoing synchronization dependency.

Start from a downloadable JSON backup. Inspect its coverage and use a read-only SIMKL API import only if necessary. CSV alone is insufficient for faithful episode-gap migration.

Import statuses, episode progress, personal ratings, custom lists, and recorded watched dates. Preserve unknown dates as unknown. Do not turn inferred completed-show episodes into falsely precise watch timestamps.

Every import has a preview of additions, changes, conflicts, unresolved mappings, and errors before application. Apply resolved items while retaining ambiguous ones in a persistent review queue.

Existing app edits win on repeat imports. Add missing history without deleting app records. Show conflicts for review. Repeating the same import must not duplicate identical history or list entries.

Import custom list memberships as manual lists unless original smart rules translate exactly. Preserve unsupported source fields and untranslated rules in the original import archive.

Do not implement import-specific undo. Require backup/export before applying an import, with the workflow making that prerequisite visible.

## 12. Portability and recovery

Provide per-user UI export and restore of library, history/dates, ratings, statuses, lists/rules, and user settings. Validate restores, preview changes, and use the same app-edits-win conflict behavior.

Account authorization remains governed by OIDC, not by roles stored in an imported user export. Do not export server secrets or authentication sessions as portable library data.

Infrastructure handles scheduled database backups and instance-wide restoration. The app does not need a separate backup scheduler or storage service in the first release.

## 13. Refresh and degraded operation

Fetch metadata on demand and cache it in PostgreSQL. Prioritize saved and active content rather than mirroring the entire catalog.

Refresh defaults:

- Active shows: daily.
- Other saved titles: weekly.
- Streaming availability: daily for recently used countries.

Provide manual refresh and visible freshness timestamps. Background failures should be retryable and visible without blocking unrelated tracking operations.

Cached library metadata and tracking remain usable when a provider is unavailable. Uncached search or details show a clear retryable error. External artwork may still be unavailable unless cached; do not claim offline media availability that does not exist.

## 14. Architecture and deployment

- Go backend with standard HTTP routing, `pgx`, and `sqlc`.
- React and TypeScript frontend built with Vite.
- Tailwind CSS and accessible UI primitives.
- TanStack Query for API state, caching, and mutations.
- PostgreSQL for users, catalog cache, history, statuses, lists, mappings, imports, and background-job state.
- One application container serving the built frontend and API and running background jobs.
- Docker Compose and Helm chart deliverables, with database connection configuration supporting an external PostgreSQL service.

No Redis, dedicated search engine, message broker, runtime LLM service, or separate worker service is required by this design. Introduce additional infrastructure only through an explicit design change.

## 15. Acceptance and validation

Validate the critical workflows with backend tests and browser-level checks:

- OIDC admission, admin claims, refresh revalidation, and user isolation.
- Search, details, library filtering, and provider selection.
- Single and bulk tracking, date precision, earlier-episode prompts, and status transitions.
- Custom-status locking and safe deletion.
- Nested smart-list logic, unknown values, dependencies, and cycle rejection.
- Alternate anime views, ambiguous mappings, split/combined consent, and history preservation.
- SIMKL preview, repeat-import idempotency, conflict handling, unresolved-item review, and source preservation.
- Per-user export/restore and documented database recovery.
- English and Turkish layouts, desktop/phone behavior, keyboard access, and visible error/empty/loading states.
- Compose and Helm deployment smoke checks and operation during provider outages.

Use synthetic or redacted data in committed test fixtures. Do not commit real user histories or credentials.

## 16. Explicitly deferred

- Automatic Plex/Jellyfin tracking and streaming-site browser tracking.
- Browser extensions and native mobile applications.
- Offline editing.
- Social features, sharing, public profiles, invitations, and open registration.
- Custom recommendation engines.
- Subscriptions and payment processing.
- External email/push notification delivery and iCal subscriptions.
- Separate rewatch sessions and season/episode personal ratings.
- Guaranteed playback deep links, episode-level streaming coverage, and dub/subtitle availability.
- Ongoing or bidirectional SIMKL synchronization.
- An exhaustive locally indexed external catalog.

## 17. Remaining verification prerequisites

These are implementation checks, not hidden feature decisions:

- Verify current AniDB access, client registration, rate, cache, and attribution requirements.
- Inspect a real private SIMKL JSON export to establish exact field coverage without publishing it.
- Verify the configured OIDC provider returns usable current admission/admin claims during refresh or through UserInfo.
- Exercise mapping coverage on representative ordinary, split-cour, special, and split/combined anime examples.
- Pin compatible dependency versions and dataset versions at implementation time.

The earlier rough AI-time and subscription-allowance estimates predated this expanded release and are not delivery commitments for this specification.
