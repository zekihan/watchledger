# Remaining first-release feature audit

This audit replaces the earlier narrow-pass completion assessment. Passing CI for one increment is not evidence that all independently actionable work is finished. Work below stays with implementation, not in the operator handoff. The contract remains [the specification](specification.md); deferred post-first-release features are not silently added.

## Current verified baseline (2026-10-25)

The three repository-audit increments are merged:
[PR 15](https://github.com/zekihan/watchledger/pull/15),
[PR 16](https://github.com/zekihan/watchledger/pull/16), and
[PR 17](https://github.com/zekihan/watchledger/pull/17).
Main at `b8602c3bbaf4a242f3bd6323c17b00f0a4ab736a` passed
[CI 40143529869](https://github.com/zekihan/watchledger/actions/runs/40143529869),
including Chromium, Firefox and WebKit. Earlier pending-CI statements below
are chronological records, not current failures at that revision.

The next local hardening increment checks complete bounded JSON request bodies,
explicit nullable library PATCH fields, and serialization of per-title structure
changes with other account writes. Its regression tests do not establish hosted
CI acceptance until the new revision is published and tested there.

Local validation of this increment on 2026-10-25 passed `make lint test build
test-race`, with PostgreSQL-backed API tests and 98 frontend tests. Regression
cases first demonstrated acceptance of trailing/oversized JSON suffixes, failure
to clear an explicit null override, and a structure write bypassing the account
lock. They now verify rejection before status/import writes, preservation of
omitted fields, explicit override clearing, and cancellation without mutation
while another account write holds the lock. No migration or dependency change
is required. Hosted CI, real-browser execution of this revision, and deployment
have not been performed.

Physical-device, real secondary-identity, private-provider and deployment
acceptance remain separately open in [TODO.md](../TODO.md). Completed synthetic
checks do not authorize deployment or replace those operator-dependent checks.

## Independent implementation audit

Checked audit items mean their implementation/evidence reconciliation is recorded below. Publication is not accepted until the latest branch CI is successful; current and prior browser failures are tracked explicitly in this record.

- [x] Dashboard: use the selected timezone/language for timestamped activity and upcoming releases, preserve date-only values, and test ordering controls including cancelled drag and concurrent saves.
- [x] Catalog credits: make the full supplied cast browsable instead of silently stopping at twenty; handle duplicate person/title credits safely and test person filmography filtering/sorting/empty states.
- [x] Lists: exercise nested AND/OR/NOT and unknown-value editing through the UI, not only backend evaluation; check dependent-list workflows and accessible editor behavior.
- [x] Library/navigation: verify custom/base status filters, sorting, keyboard shortcuts and visible equivalents against synthetic populated UI fixtures; fix gaps found.
- [x] Localization/settings: verify browser-language fallback, separate settings and date formatting across all visible screens; distinguish untranslated source content from UI strings.
- [x] Tracking/detail: verify movie/bulk chosen/unknown dates, ended-show transitions, specials/spoilers and retained source history in UI fixtures alongside existing database tests.
- [x] Discovery/availability: verify filters, person-credit pagination, selected-service behavior, no-match/error states and attribution in UI fixtures.
- [x] Import/portability: reconcile every tracker claim with existing HTTP/database/UI tests, add missing independent coverage and keep only actual private-provider/device acceptance owner-dependent.
- [x] Background operations: verify weekly saved-title and recently-used-country refresh schedules independently, plus user-visible admin error/retry states.
- [x] Reconcile every unchecked first-release tracker item with implementation, named test evidence, an actual independent task, or a specifically justified operator-only acceptance item. Do not blanket-check visual acceptance based on jsdom.

## Existing evidence, not a substitute for this audit

- `033df6b` and `a203033` passed full local lint, database-backed tests, frontend tests, build and race checks. Exact-head CI also passed synthetic Compose and Helm validation.
- Earlier real-provider and Mac browser/runtime evidence is in [the dated acceptance record](acceptance-2026-10-24.md). Preserve it rather than repeating completed setup.
- Current code exists for each major first-release surface. Many remaining checkboxes mix implementation and acceptance; their existence alone neither proves absence nor completion.
- Real secondary identity/group changes, physical Android installation, Mac Docker/private-issuer transport, private source edge cases and genuinely ambiguous edition decisions remain in [TODO.md](../TODO.md). Ordinary test gaps, browser-independent accessibility work and UI fixes do not belong there.

## Evidence from the active audit

### Dashboard and catalog

- `dashboard.test.tsx` verifies selected-zone timestamp dates, unchanged date-only values, unknown-date fallback, source-safe alternate links, move-up/down persistence, pending-write guards, cancelled drag, and explicit preview/apply for the new quick-watch control. The shared tracking flow retains chosen/today/unknown dates and consent dialogs.
- `filmography.test.tsx` verifies supplied-credit pagination, deduplication by content type plus ID, movie/TV separation, role/type filters, release/popularity sorting, empty matches and filter page resets. `native-credits.test.tsx` now verifies access beyond the first twenty cast entries; native creator identities remain separate.
- Shared date formatting now applies the UI language/timezone rather than metadata language or host defaults to metadata freshness, admin retry/history timestamps, title-index import time and list-fetch freshness.
- `library-locale.test.tsx` verifies exact custom statuses alongside mapped base filters, content-type filtering, Turkish dotted/dotless-I search, whitespace handling, Turkish browser initialization and English fallback. Separate metadata/UI settings are preserved.
- `lists.test.tsx` now drives nested OR/NOT and explicit unknown rules through the real visual editor and checks the saved structure. Backend unknown/dependency/cycle coverage remains in `internal/smart/rules_test.go` and `internal/httpapi/list_results_test.go`.
- `internal/jobs/cadence_test.go` verifies daily active-title, weekly other-saved-title and daily recently-used-country queue eligibility, no unsaved catalog mirror, fresh/stale cutoffs and idempotent enqueueing without live providers.

### Watched exclusions

The audit found that discovery equated Completed status with watched history. The new authenticated `/api/library/watched` query uses private recorded/effective facts instead: manually completed entries without history stay visible, watched films remain excluded even after a manual status change, and series completion requires an ended/cancelled source and the full declared regular-episode cache. Caught-up ongoing shows stay visible. Confirmed movie aliases participate; unconfirmed links, explicit unwatch decisions and incomplete caches never manufacture completion. No metadata fetch or write is triggered. `watched_test.go` covers these cases and account isolation; authentication rejection is covered in `library_test.go`. The UI waits for/retries the history lookup and explains conservative exclusion in both languages.

### Actual browser boundary

A fresh command-workspace Chromium attempt failed before page loading at `socket() failed: Operation not permitted` in its process-singleton initialization. No sandbox/security bypass or protected-directory access was attempted. This is not evidence of app behavior and does not block the independent code and synthetic interaction audit. Existing Mac browser evidence remains valid for its recorded revision; new real-browser acceptance remains distinct.

## Browser CI evidence and continuing coverage

A pinned Playwright suite now targets the existing disposable Compose app with real synthetic database/API state, in English/Turkish at desktop and phone-sized viewports. It covers restore, selected-episode/unknown-date tracking, default/cancel/focus behavior, manual/smart lists, export exclusions, navigation and overflow. The nine-case suite passed in run `40046909127`; newer restore/calendar changes still require their own exact-head result. Type-checking or test discovery alone does not complete acceptance. Screenshots contain only synthetic fixtures and can support visual review after execution. This work is independent and must not be moved to the operator checklist merely because local Chromium cannot launch.

### Continued tracking, administration and browser work

- Empty chosen-date inputs now fail before preview/apply instead of becoming implicit unknown-date events. The user must choose a date or explicitly select Unknown. Detail and shared quick/alternate tracking paths have regressions; pending controls cannot change the selected date mid-action.
- Admin mapping/history, account oversight, background failure queries and private import archive/review queues now have accessible loading/error/retry feedback. Non-admin screens never request account/job oversight. `admin-recovery.test.tsx` tests independent recovery and absence of admin requests.
- Daily metadata refresh is limited to actively watched episodic titles; saved movies use the weekly cadence even if manually marked Watching. The cadence regression distinguishes the two.
- Catalog content-type labels now use the interface translation instead of exposing raw English type identifiers in Turkish layouts.
- The first actual Chromium CI run reached the app but exposed a test-session handoff bug: same-document hash navigation retained the already-rendered anonymous query state. The screenshot confirmed the login page, not a missing settings screen. The harness now starts a fresh document after adding its synthetic session. This does not weaken app authentication or a test assertion.
- The synthetic issuer now provides two fixed, distinct test identities with subject-preserving refresh tokens. A separate verified OIDC flow obtains the second session; new browser checks attempt cross-account library/list/history/import access without changing database roles. Real provider/group-change acceptance remains distinct.

### Large-import and localization follow-through

Source actions, archive states and known import warnings now have readable English/Turkish labels; unknown diagnostic codes and unsupported-section identities are retained rather than guessed away. Preview records and unresolved queues are paged in groups of fifty without truncating application: the interface explicitly states that Apply processes the entire preview. Regressions cover page boundaries and the unchanged full-import request. Smart-rule tests now exercise each supported field's known/unknown behavior, numeric boundaries and nested truth tables; UI coverage includes stable smart-list membership and named deletion blockers.

The second browser run passed English desktop/phone workflows, Turkish phone workflows and cross-account isolation. The Turkish desktop keyboard test could attempt focus before its tracking button became enabled; the harness now waits for the action to be enabled and verifies focus before pressing W. It keeps the same dialog/default/cancellation assertions. Successful synthetic screenshots were reviewed; this led to a footer separator correction and explicit empty import-queue states. Browser fixtures now include regular episodes plus dated/timestamped specials so calendar/upcoming layout checks are not limited to empty screens. Manual-list editing no longer presents a misleading smart-list sort control; manual order remains explicit member order.

## Contract reconciliation by feature family

This section records implementation evidence rather than using a remaining browser/provider checkbox to imply missing code. Visual/device assertions still need their own evidence.

| Feature family                                                                        | Implementation and independent evidence                                                                                                                                                                        | Remaining distinction                                                                                                                                                                |
| ------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Identity, admission, admin, refresh, privacy                                          | `internal/auth/*_test.go`, `internal/httpapi/library_test.go`, `simkl_test.go`, `list_results_test.go`; real synthetic two-identity OIDC/browser isolation now runs in CI                                      | Real provider-side group removal and disposable real secondary accounts remain operator-deferred.                                                                                    |
| Navigation, dashboard, library, keyboard controls                                     | `App.test.tsx`, `dashboard.test.tsx`, `library-locale.test.tsx`; browser suite exercises persistent navigation, W, Escape, focus return and header search                                                      | Nine-case browser execution passed; reviewed screenshots produced follow-up fixes. Updated screenshots and physical-device behavior remain separate.                                 |
| Search and movie/TV/native catalog                                                    | `internal/catalog/tmdb_test.go`, `internal/anidb/client_test.go`, `titles_test.go`, `hot_test.go`, `internal/httpapi/anime_test.go`; real TMDB/AniDB acceptance already recorded                               | Native source limits are explicit; do not fabricate biographies, upcoming feeds or genre equivalence.                                                                                |
| Details, seasons, episodes, artwork, cast, crew, related titles, trailers             | `episode-details.test.tsx`, `native-detail.test.tsx`, `native-credits.test.tsx`, `images.test.ts`; detail renders supplied recommendations and request-only YouTube links                                      | Source-provided optional fields can be absent. No autoplay or playback guarantee is claimed.                                                                                         |
| Person pages and filmography                                                          | `filmography.test.tsx`, `native-credits.test.tsx`; source-specific native creator route, supplied biography/image and movie/TV/anime credits                                                                   | Cached AniDB filmography is explicitly non-exhaustive.                                                                                                                               |
| Discovery filters and pagination                                                      | `discovery.test.tsx`, `internal/catalog/tmdb_test.go`, `people_discovery_test.go`, `internal/httpapi/watched_test.go`                                                                                          | People-based TV discovery reports candidate-page limits, not exhaustive totals; watched filtering is conservative.                                                                   |
| Country, subscriptions, availability and attribution                                  | `availability-retry.test.tsx`, `credits.test.tsx`, `internal/catalog/cache_test.go`, `people_discovery_test.go`; accepted real-provider checks and approved logo                                               | Missing offers stay unknown/unavailable; supplied availability links are not invented playback links.                                                                                |
| Statuses, ratings, automatic progress transitions                                     | `internal/httpapi/library_test.go`, `linked_status.go`, `internal/tracking/rules_test.go`, `status-replacement.test.tsx`                                                                                       | Native entries retain their own ratings/statuses. Manual overrides and source uncertainty remain explicit.                                                                           |
| Movie, individual/bulk episode tracking and precision                                 | `library_test.go`, `effective_test.go`, `movie_mapping_test.go`, `tracking-ui.test.tsx`, `episode-details.test.tsx`, `alternate-tracking.test.tsx`                                                             | Existing known dates survive bulk actions; new unknown dates require explicit selection rather than an empty chosen-date input.                                                      |
| Up next, calendar, recent/upcoming, spoilers, notices                                 | `upnext_test.go`, `dashboard.test.tsx`, `analytics.test.tsx`, `episode-details.test.tsx`, `notices.test.tsx`                                                                                                   | Calendar date-only/timestamp precision is tested; browser fixtures now populate specials and upcoming releases.                                                                      |
| Manual/smart lists, nested logic, unknowns, membership, cycles, dependencies, sorting | `lists.test.tsx`, `internal/smart/rules_test.go`, `internal/httpapi/list_results_test.go`, `library_test.go`                                                                                                   | Manual lists use member order; smart lists use saved sorting. Provider-service facts are evaluated in the selected country, not inferred from missing offers.                        |
| Anime views, native identities, consent, mapping revisions and corrections            | `alternate_test.go`, `effective_test.go`, `datasets_test.go`, `movie_mapping_test.go`, `statistics_native_test.go`, `alternate.test.tsx`, `mapping-review.test.tsx`; representative real-source cases recorded | Ambiguous editions stay unresolved, not guessed or declared broken code. New source/edition decisions require evidence.                                                              |
| SIMKL backup/API import, preview, archive, conflicts, repeat import, review queue     | `simkl_test.go`, `import_review_test.go`, `import_catalog_test.go`, `custom_list_import_test.go`, `simkl_api_test.go`, `imports.test.tsx`, `preview-navigation.test.tsx`                                       | The inspected backup has no custom lists. Verified list-response membership is a manual snapshot; unverified automatic-rule semantics remain archived, not translated speculatively. |
| Portable UI export/restore, settings, lists/rules, date/history and isolation         | `internal/portable/snapshot_test.go`, `library_test.go`, `custom_list_import_test.go`, `imports.test.tsx`, real-browser restore/export and second-account checks                                               | Library restores cannot restore role/session authority. Actual operator recovery evidence is preserved separately.                                                                   |
| Statistics and alternate-view deduplication                                           | `statistics_test.go`, `statistics_native_test.go`, `analytics.test.tsx`, accepted real-source mapping exercises                                                                                                | Unknown dates and partial/unconfirmed source units stay separately labelled.                                                                                                         |
| Daily/weekly metadata, country availability, retries, cached operation                | `internal/jobs/anime_test.go`, `cadence_test.go`, `runner_test.go`, `internal/catalog/cache_test.go`, `admin-recovery.test.tsx`, `metadata-refresh.test.tsx`                                                   | Synthetic cadence/outage tests are independent; a deployed scheduler observation is still environment acceptance.                                                                    |
| Architecture, packaging, deployment and recovery                                      | Standard HTTP/pgx/sqlc, React/TypeScript/Tailwind/Query; exact-head lint/build/race/DB tests; synthetic Compose and accepted disposable Helm/recovery evidence                                                 | Actual Mac Docker-to-private-issuer transport is not an app implementation task. No homelab deployment is implied.                                                                   |

Catalog-presentation browser cases additionally cover rendered synthetic artwork, search-result arrow navigation, movie synopsis/credits/related titles, subscription/rental/purchase links, no automatic video/embed creation, person biography and credit filters, and discovery runtime/cast/crew/selected-service query propagation in both languages and viewport sizes. These cases explicitly mock only provider responses; they supplement, not replace, the recorded real-provider adapter checks.

### Native portable-restore cache gap

The audit found a real recovery gap: native catalog/episode metadata was exported, but AniDB detail reads previously required the separate provider XML document, which portable exports intentionally do not contain. A restored native entry could therefore fail to open without fresh provider access despite having usable cached metadata and history.

AniDB detail now falls back to the exact AniDB catalog identity and its non-retired stored episode metadata when no XML document exists. It honors title preference/language, preserves the original freshness timestamp, makes no source reservation for a fresh restored cache, and labels stale fallback on provider unavailability. It does not invent metadata, change history, create XML, reset rate limits or use another provider's identity. `internal/anidb/portable_cache_test.go` exercises fresh/stale restored reads, title style, source episode retention, timestamp preservation and an uncached failure. Real-browser synthetic restore cases now also open and track a restored native entry with no AniDB credentials; their exact CI result remains required.

### Real-browser visual findings

CI [40046909127](https://github.com/zekihan/watchledger/actions/runs/40046909127) passed all nine Chromium cases on `5aeb132`, including both interface languages at desktop/phone sizes, catalog presentation and two-account isolation. Source responses in catalog presentation are explicitly synthetic; they do not replace the separate recorded live-provider acceptance.

Screenshot review found a genuine phone calendar readability issue despite passing overflow checks: seven columns reduced release titles to a few characters per line. Phones now start in the readable agenda view; the optional month grid retains readable cell widths inside a keyboard-focusable horizontal scroller. The same review exposed a stale provider error from Settings on the list index even though no service rule was being edited. Provider failures now appear only while the relevant selector is active. Both findings have independent regressions, and browser checks retain both calendar views. Updated visual verification is still pending the next exact-head run.

The tracker now marks individually verified implementation behaviors using the family evidence above, while leaving composite visual/accessibility, physical-device, real-account/source and deployment acceptance open. Conditional source smart-rule translation remains open because no verified source automatic-rule schema has been supplied; rules remain archived, never guessed. Cross-account browser coverage is being extended from isolation to portable restore/reimport with retained private edits and unknown dates.

Automated accessibility scans using pinned `@axe-core/playwright` are being added to the populated main-route and movie-detail browser matrix. These check WCAG A/AA rules with no rule exclusions; they supplement rather than replace keyboard/manual assistive-technology checks. Review also named statistics meters and raised the low-contrast sidebar section label. Scan execution must pass before this work is counted as accepted.

The native portable-cache change passed [CI 40047729763](https://github.com/zekihan/watchledger/actions/runs/40047729763) at `b6fdab9`, including real-browser restored native detail and unknown-date tracking with provider credentials absent. The next browser increment adds a third, explicit synthetic admin identity from the same CA-verified OIDC fixture, source-safe mapping create/review/history and private-account oversight checks, plus SIMKL conflict/reimport/unresolved-archive UI coverage. The test-only issuer is not included in the application binary/image and does not modify database roles or production identity settings.

The first axe-enabled run (`40048373216`) found footer links distinguished only by color. They now have persistent underlines; no accessibility rule was suppressed. The cross-account restore test correctly hit the server's current-export prerequisite, so the browser flow now actually downloads that account's backup before applying, rather than merely ticking the checkbox. Reviewed phone calendar screenshots confirm readable agenda entries. Their mixed date/timestamp fixtures also exposed chronological ordering gaps: calendar and upcoming queries now sort by selected-zone day and then actual timestamp, with stable identity ties and unchanged date-only precision. Database regressions cover both endpoints.

A TypeScript AST review of visible JSX text and literal accessible labels found untranslated mapping-release and AniDB-identity labels, plus the English anime-title-style option. These now use the interface translations. Remaining literal text consists of brand/license names, protocol identifiers and language autonyms; provider metadata/diagnostics remain source content. This complements the English/Turkish browser matrix rather than relying on translation-key parity alone.

At `96067fd`, [CI 40049102769](https://github.com/zekihan/watchledger/actions/runs/40049102769) passed ten browser cases, including all populated main-page/movie accessibility scans, second-account restore/reimport and the synthetic admin mapping workflow. The final SIMKL case applied safely and displayed the unresolved source record, confirmed in its screenshot; its exact-text locator incorrectly included the adjacent action button. It now locates the named review action within that source row. No product safeguard or assertion about retained history was removed. The mapping page's stale unconditional “source verification pending” banner was replaced by the actual identity/review policy in both languages.

### Remaining open tracker classifications

- Responsive/visual/accessibility composite items combine implemented layouts and verified browser behavior with intended physical-device or assistive-technology acceptance. Existing 1440px desktop and 360/390px phone-sized Chromium evidence is recorded above; physical Android and screen-reader checks stay in TODO.md. Automated accessibility scans do not prove universal conformance.
- Native/alternate-view and aggregate-history composite items have implemented source-safe behavior, database/UI regressions and the recorded representative real-source mapping exercises. Broader edition coverage is limited by source evidence, not missing generic tracking code; the specifically unresolved edition decision stays in TODO.md.
- Live AUTH V2/list-source composites retain the already completed real-adapter and representative browser acceptance. Premium/expired-token/other-account and full private-backup cases still require those actual inputs, not a replacement implementation.
- Exact translation of source automatic rules is conditional on a verified schema/equivalence. No such schema was present in the inspected source; manual snapshots and preserved raw rules are deliberate, not inferred smart rules.
- Daily live refresh and real-provider Compose acceptance require the intended running environment and its issuer connectivity. Scheduling logic, retry behavior and synthetic Compose execution are tested independently.
- Final branch acceptance requires successful exact-head CI, including the remaining SIMKL repeat-import case and refreshed screenshots. A failed or unrun browser assertion remains independent work until fixed; it is never an operator blocker.

## Acceptance hardening continuation (2026-10-24)

The merged first audit passed all eleven Chromium browser cases in
[main CI 40053686370](https://github.com/zekihan/watchledger/actions/runs/40053686370)
at `01584df`. Earlier pending statements above are chronological records, not
current failures. This does not exhaust independently actionable verification.

The next increment adds isolated Chromium, Firefox and WebKit CI jobs, bulk
tracking/date-preservation and custom-status replacement/cancellation checks in
all four language/viewport scenarios, and real application browser callbacks with
synthetic denied/revoked identities. The short-lived synthetic identities change
claims only on refresh; there is no provider management endpoint or database role
edit. The ephemeral issuer CA remains verified. These new checks are pending
exact-head CI and must not be counted as passed yet.

Still independent: expand split/combined and alternate-view browser tracking,
review cross-browser findings and reconcile final evidence. Real accounts,
physical devices and deployment remain separate boundaries.

The first three-browser run (`40122621208`) passed catalog presentation,
synthetic admin mapping, denied admission and admission removal, but exposed test
setup defects: the new bulk-action selector matched both direct and alternate
date controls, and the new admin-refresh settings request omitted required
`anime_view`. The direct control is now explicitly scoped and settings remain
subject to normal validation. SIMKL acceptance now saves its own current backup
rather than depending on the preceding cross-account test. No product safeguard
or assertion is removed. Combined native-parts/alternate tracking and signed
fixture claim-transition tests are included in the next run.

### Verified hardening result

At `9a5f37c`, [CI 40123390744](https://github.com/zekihan/watchledger/actions/runs/40123390744)
passed all fifteen scenarios in each of Chromium, Firefox and WebKit: **45 browser
executions**, with no retries or suppressed accessibility rules. The exact-head
run also passed 89 frontend tests, Go/database/race tests, lint, builds, synthetic
Compose login/tracking/restart and Helm validation. All three jobs succeeded.

The new combined-episode case uses two exact synthetic native parts and one
combined TV episode. It verifies unconfirmed blocking, consent dismissal, explicit
confirmation, incomplete versus complete progress, linked-unwatch default and
Escape/focus return, explicit linked removal, unchanged native rating and no
fabricated aggregate library entry. The authentication cases verify denied
admission, loss of admission at refresh and loss of admin while preserving the
admitted subject and settings. These do not impersonate real users or prove a
particular external identity provider's behavior.

Reviewed Firefox phone library/settings and WebKit Turkish phone calendar/catalog
screenshots after the successful run. The sampled screens remain readable, with
wrapping, chronological agenda entries and no document-width overflow. Desktop
WebKit is not physical Safari/iOS acceptance, and Firefox phone-sized windows
are not Android device acceptance.

The enumerated hardening pass is verified. No product defect was established by
its failures: fixture selectors and required settings/backup setup were corrected
without weakening production validation. Remaining owner-dependent acceptance is
in TODO.md. This is not a claim that no future bugs, broader catalog cases or
additional useful tests can exist, nor does it authorize merging or deployment.

## Follow-on bug audit fixes

The owner requested fixes for the separate 2026-10-24 repository audit. This
increment addresses the nine reproduced defects and the dependency finding:

- Gate new shared restore metadata using an installation proof or explicit
  administrator review; reject future cache freshness.
- Preserve watch/unwatch intent across portable backups without allowing
  imported records to override destination-local decisions (migration 018).
- Remove the globally saturatable pending-login pool, retaining encrypted
  browser-bound state, nonce, PKCE and expiry checks.
- Decode TMDB person TV credits with explicit cast/crew fields, accepting the
  documented numeric root ID.
- Disable overlapping status/rating edits until the save and refresh finish.
- Remap nested custom-status predicates during restore.
- Block custom-status deletion with named dependent smart lists rather than
  silently leaving broken predicates.
- Align successful portable export and restore limits at 64 MiB, with an explicit
  database-backup fallback for larger libraries.
- Reconcile episode kind when provider records move into or out of specials.
- Upgrade golang.org/x/text to v0.41.0; the local follow-up govulncheck reports no
  vulnerabilities.

Regression tests replace the audit reproductions. Browser fixtures use explicit
synthetic administrator approval for their fabricated shared catalog; the actual
ordinary-user restore and cross-account privacy assertions remain intact.
Exact-head CI and final verification are still required before completion.

## Second repository audit fixes

The second pass targets ten independently reproduced issues on `81b261b`:

- Account identity changes hide private screens immediately, cancel outstanding
  private queries, clear their caches and mutation state, and remount forms.
  Tests include an old response arriving after the account switches.
- Portable upload validates JSON but sends the original bytes, preserving signed
  decimal representations such as `8.0` instead of normalizing them to `8`.
- Native portable-cache fallback rebuilds episode presentation identity, numbering,
  kind and date from authoritative stored columns, not incomplete metadata JSON.
- Successful complete TMDB refreshes retire vanished seasons and invalidate their
  season caches while preserving history and issuing reconciliation notices.
- Shared timezone validation rejects host-dependent `Local` and zoneinfo tree
  pseudo-names. Migration 019 normalizes previously accepted values to UTC.
- Migration 019 also records ordinary-list deletions. Portable backups carry those
  facts; additive restore preserves them and previews skipped dependent rules.
- Smart-list save and preview revalidate custom-status ownership/existence;
  restore validation also rejects dangling custom-status predicates.
- Malformed successful TMDB responses fall back to usable cached detail with an
  explicit stale warning, without storing the invalid response.
- Person-based TV discovery sorts supplied credit candidates before paging and
  applies excluded genres; unsupported sorting is rejected rather than ignored.
- Calendar and dashboard titles use each account's selected metadata language.

Regression coverage is in the second-audit HTTP/job tests, portable/timezone
validation, account-boundary and raw-upload UI tests, plus a same-tab identity
change browser case. Browser and publication acceptance require exact-head CI;
local jsdom tests alone do not establish them. Physical-device and live-provider
acceptance remain separately tracked in TODO.md.

Local final-code verification passed lint, PostgreSQL-backed tests, 94 frontend
behavioral tests, browser-test type checking, production build and Go race tests.
The new real-browser case still requires exact-head CI; no deployment is implied.

## Third audit: edit safety, restore compatibility and bounded work

The third audit of main `db4b83e` produced six reproduced bugs and four
functional/scalability improvements. The implementation on
`fix/third-audit-findings` addresses all ten:

- Existing-entry edits use field-specific PATCH requests under the account lock.
  Status changes do not resend stale ratings, and rating changes do not resend
  stale statuses. Explicit null clears a rating; omitted fields remain unchanged.
  PUT remains the full-record creation/replacement API for existing clients.
- Restore plans reuse compatible custom status names, preserve existing edits,
  remap incoming references and expose incompatible base-status collisions before
  apply. Apply rechecks the plan under the account lock; it never guesses a base.
- Season outage fallback excludes retired episodes without deleting history.
- Malformed availability envelopes/offer structures preserve cached offers with a
  stale warning instead of recording a false fresh negative result.
- Smart rules distinguish missing fields from known-empty genres and cached
  negative availability. Free and ad-supported offers are included consistently
  in service filters and displayed separately in details.
- Invalid encoded navigation links show a recoverable translated error while
  retaining the application navigation.
- Library responses carry compact card/filter metadata instead of full detail
  payloads. The UI renders 48 entries per page, with filtering/sorting across the
  full summary collection and page reset/clamping after filtering or refresh.
- Restore preview uses one set-based library lookup, including a 10,000-candidate
  regression, rather than one query per incoming entry.
- The HTTP server bounds body reads to five minutes and writes to six minutes.
  Ordinary handlers have a two-minute cancellation budget; restore, SIMKL, admin
  and export handlers have five minutes. Proxy budgets must accommodate intended
  large imports without disabling the application's bounds.

Regression coverage includes cross-account PATCH rejection, nullable fields,
status-name collision preview/apply, known-empty smart conditions, free offers,
retired fallback, malformed availability, compact payloads, 10,000-candidate
lookups, request cancellation, invalid routes, 1,000-entry UI pagination and
field-only mutation payloads. Tests use synthetic data. Real-browser acceptance
and exact-head hosted CI must be recorded separately from local jsdom results.

Local verification on 2026-10-24: `make lint test build test-race` passed with
PostgreSQL-backed tests and 98 frontend tests. These are local API/database,
jsdom, type-check, build and race-detector results, not hosted CI or a deployment.
