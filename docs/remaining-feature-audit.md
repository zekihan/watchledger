# Remaining first-release feature audit

This audit replaces the earlier narrow-pass completion assessment. Passing CI for one increment is not evidence that all independently actionable work is finished. Work below stays with implementation, not in the operator handoff. The contract remains [the specification](specification.md); deferred post-first-release features are not silently added.

## Independent work queue

- [x] Dashboard: use the selected timezone/language for timestamped activity and upcoming releases, preserve date-only values, and test ordering controls including cancelled drag and concurrent saves.
- [x] Catalog credits: make the full supplied cast browsable instead of silently stopping at twenty; handle duplicate person/title credits safely and test person filmography filtering/sorting/empty states.
- [ ] Lists: exercise nested AND/OR/NOT and unknown-value editing through the UI, not only backend evaluation; check dependent-list workflows and accessible editor behavior.
- [ ] Library/navigation: verify custom/base status filters, sorting, keyboard shortcuts and visible equivalents against synthetic populated UI fixtures; fix gaps found.
- [ ] Localization/settings: verify browser-language fallback, separate settings and date formatting across all visible screens; distinguish untranslated source content from UI strings.
- [ ] Tracking/detail: verify movie/bulk chosen/unknown dates, ended-show transitions, specials/spoilers and retained source history in UI fixtures alongside existing database tests.
- [ ] Discovery/availability: verify filters, person-credit pagination, selected-service behavior, no-match/error states and attribution in UI fixtures.
- [ ] Import/portability: reconcile every tracker claim with existing HTTP/database/UI tests, add missing independent coverage and keep only actual private-provider/device acceptance owner-dependent.
- [ ] Background operations: verify weekly saved-title and recently-used-country refresh schedules independently, plus user-visible admin error/retry states.
- [ ] Reconcile every unchecked first-release tracker item with implementation, named test evidence, an actual independent task, or a specifically justified operator-only acceptance item. Do not blanket-check visual acceptance based on jsdom.

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

## Browser CI work in progress

A pinned Playwright suite now targets the existing disposable Compose app with real synthetic database/API state, in English/Turkish at desktop and phone-sized viewports. It covers restore, selected-episode/unknown-date tracking, default/cancel/focus behavior, manual/smart lists, export exclusions, navigation and overflow. Browser execution is still pending the exact-head CI result; type-checking or test discovery alone does not complete acceptance. Screenshots contain only synthetic fixtures and can support visual review after execution. This work is independent and must not be moved to the operator checklist merely because local Chromium cannot launch.

### Continued tracking, administration and browser work

- Empty chosen-date inputs now fail before preview/apply instead of becoming implicit unknown-date events. The user must choose a date or explicitly select Unknown. Detail and shared quick/alternate tracking paths have regressions; pending controls cannot change the selected date mid-action.
- Admin mapping/history, account oversight, background failure queries and private import archive/review queues now have accessible loading/error/retry feedback. Non-admin screens never request account/job oversight. `admin-recovery.test.tsx` tests independent recovery and absence of admin requests.
- Daily metadata refresh is limited to actively watched episodic titles; saved movies use the weekly cadence even if manually marked Watching. The cadence regression distinguishes the two.
- Catalog content-type labels now use the interface translation instead of exposing raw English type identifiers in Turkish layouts.
- The first actual Chromium CI run reached the app but exposed a test-session handoff bug: same-document hash navigation retained the already-rendered anonymous query state. The screenshot confirmed the login page, not a missing settings screen. The harness now starts a fresh document after adding its synthetic session. This does not weaken app authentication or a test assertion.
- The synthetic issuer now provides two fixed, distinct test identities with subject-preserving refresh tokens. A separate verified OIDC flow obtains the second session; new browser checks attempt cross-account library/list/history/import access without changing database roles. Real provider/group-change acceptance remains distinct.
