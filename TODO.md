# Operator involvement before first-release acceptance

This is the handoff list for work that cannot be completed independently in the current workspace. It is **not** a declaration that all product features are finished. Independently implementable work stays in [the feature tracker](docs/features.md); do not move ordinary coding or fixable test failures here to call the release complete.

Do these checks after the remaining implementation increments are pushed. Use a disposable local installation with synthetic history first. Record the tested commit, environment, result, and any redacted failure evidence beside each item. Do not commit credentials, private source exports, database dumps, or real viewing history. Nothing here authorizes a homelab deployment, new credential issuance, or merging the feature branch.

## Required asset

- [x] Include an approved TMDB logo from the [official logo gallery](https://www.themoviedb.org/about/logos-attribution). Retrieved the official Alt short (blue) SVG through Chrome on 2026-10-23; the unchanged asset is `web/public/credits/tmdb.svg`, with provenance in [data sources](docs/data-sources.md#tmdb-logo). The credits page displays it at 80 pixels wide alongside the existing non-endorsement notice. [TMDB attribution requirements](https://developer.themoviedb.org/docs/faq) were rechecked. Overall live-provider attribution acceptance remains separate.

## Existing provider accounts and configuration

- [ ] Configure a disposable OIDC client on your chosen provider using [development configuration](docs/development.md). Keep admission and admin claims separate. Verify admitted non-admin and admin accounts, denied admission, removed admission/admin claims at refresh, and current claims from a refreshed ID token or subject-validated UserInfo. Also exercise the server-side claim-configuration recovery runbook in deployment.md without changing account identity or granting a database role bypass. Synthetic authentication tests do not prove your provider's behavior. Do not paste secrets in chat or commit them.
- [ ] Configure your existing TMDB read-access token locally and run movie/TV/person search, detail, translated metadata, seasons, discovery filters, credits and country availability checks. Capture only redacted errors. Check availability links and JustWatch attribution; metadata must not fabricate unavailable offers.
- [ ] Verify AniDB use in the intended deployment fits its non-commercial/source policy. Use the registered public client name `watchledger`, version `1`, and the official title-only index. Check native titles, episode kinds and artwork against representative records. Respect the persistent two-second global and one-day per-anime limits; do not repeatedly hit the live API to test throttling.

## Private SIMKL coverage and optional live list import

- [ ] Inspect a current private SIMKL JSON backup locally against [the import acceptance steps](docs/local-acceptance.md). Confirm actual status, date, episode-gap, rating and custom-list field coverage. Keep the original backup private; share only a deliberately redacted minimal example if a mismatch needs a code fix. Unsupported and ambiguous records must remain visible in the review queue, not be guessed or silently discarded.
- [ ] If using optional direct custom-list fetching, test with your existing AUTH V2 client ID and eligible SIMKL account/token. Enter the token only in the app's temporary password field over HTTPS or loopback. Verify complete pagination, denied/premium-only errors, preview without writes, explicit apply, repeat-import behavior, original cache timestamps and account isolation. No ongoing sync, new authorization grant or credential creation is performed by the app. A complete API-response file is an alternative when live access is unavailable.

## Source-dependent coverage decision

- [ ] Review native AniDB coverage after implementation: native title search, metadata, related titles, cached creator credits and the official Hot Anime feed are supported. The selected HTTP/title-index integration does not supply full creator biographies/filmographies, a global upcoming catalog, or a verified equivalence between AniDB tags and TMDB genres. If those must be identical across providers, approve an additional documented source and its identity/attribution rules before implementation; do not invent a genre mapping or merge people by matching names/numeric IDs. Current cached-only/source-specific limits remain visible in the UI.

## Browser and device acceptance

- [ ] Run [local acceptance](docs/local-acceptance.md) in a real desktop and phone browser. Chromium cannot run in the current command workspace; jsdom tests are not browser/visual acceptance. Check both English and Turkish, narrow screens, touch controls, loading/error/empty/retry states, keyboard navigation, dialog focus/escape/return focus and visible equivalents of shortcuts. Report screenshots with private data and credentials removed.
- [ ] Verify home-screen installation and launch on your intended browsers/devices. Confirm icon, app name, start URL and authentication return behavior. Offline editing is explicitly not supported.
- [ ] Exercise ordinary, split-cour, special, split/combined and movie-alias anime mappings using explicit source IDs. Confirm native and alternate views show the same effective progress, no duplicate library ratings/statuses are invented, incomplete mappings stay usable, consent is required, changed mappings preserve history, and statistics count native units with partial/unknown dates separately. Broad live mapping coverage is not established by synthetic regressions or decoding the full dataset alone.
- [ ] With two disposable accounts, complete the tracking, custom-status, smart-list, import/reimport and portable export/restore workflows. Verify all private records remain isolated, existing edits and unwatch decisions win, unknown dates remain unknown and roles/sessions are not restored. Save an actual export file before import, not just a successful export request.
- [ ] Test provider outages after caching data. Cached tracking must remain usable; uncached searches must show retryable errors. Verify source freshness and background failure notices. Do not equate unavailable external artwork with lost history.

## Runtime and deployment acceptance

- [ ] Verify your real Compose configuration on your PC using the [deployment guide](docs/deployment.md). CI has already passed synthetic container startup/migrations, readiness, OIDC login, frontend/API serving, restore/tracking/export and restart persistence. The remaining check is your actual provider configuration, network/TLS and runtime environment, not the existence of a functioning Compose package.
- [ ] Validate the Helm chart, then smoke-test only in a disposable Kubernetes environment you explicitly choose and authorize. Supply an existing Secret and external PostgreSQL. Verify the single replica, readiness, configuration, login and cached tracking. Do not deploy to an existing homelab merely to complete the checklist.
- [ ] Verify recovery in your intended runtime: dump a disposable populated database, restore to a different empty database, then log in and inspect private history/date precision with the intended OIDC/session configuration. Physical synthetic recovery through 17 migrations has already passed in this workspace; this item is runtime/login acceptance, not a claim that recovery code is missing.

## Completion rule

An item is complete only with evidence from the tested revision. If a check exposes a code defect, return that defect to implementation, fix and retest it; do not classify it as permanently blocked merely because it was discovered on your PC. Keep the feature branch unmerged until explicitly authorized.
