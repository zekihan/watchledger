# Local acceptance evidence — 2026-10-24

Acceptance remains incomplete. This records observed results and limits, not a waiver of [TODO.md](../TODO.md) or the specification. No changes were pushed or deployed to the homelab. Credentials, private backups, database dumps, original source data and screenshots with account information remain outside Git.

## Revisions and environment

The starting implementation was `9ad8405`. The following fixes were committed during acceptance:

- `55941f7`: generate distinct custom-list fixture IDs even when consecutive clock reads have equal resolution.
- `80db82a`: display “Not rated” rather than “Unknown date” for unrated alternate entries.
- `56a2327`: configure extra OIDC scopes and explicitly select subject-validated UserInfo claims for providers such as Authelia.
- `108c23c`: log only HTTP status, whitelisted OAuth error category or error type for failed login exchanges; never token response bodies or credentials.
- `f3c090f`: capture the initiating tracking control before it becomes disabled and restore focus after pending work finishes. Real Chrome reproduced the original failure and verified Escape focus return after the fix.

Host checks used Go 1.27.3, Node 24.22.1 and a new disposable PostgreSQL 17.11 container. `npm ci` and `make lint test test-race build` passed after the OIDC behavior change, with database-backed tests enabled. Lint, database tests and build passed again after the diagnostic-only change. Earlier fixture failure was reproduced and fixed rather than bypassed.

## Authentication

Real Authelia discovery, admin login and a forced authorization refresh passed using the existing issuer/client and separate admission/admin policy. The loopback callback was registered by the operator. The operator approved identity/profile/offline access and then the expanded group-membership consent. The verified extra scope and UserInfo selection were saved in the existing ignored local configuration without printing its contents.

Refreshing an expired disposable application session replaced its encrypted token material, obtained fresh claims and retained the expected admin interface. The provider-subject binding and configured session key were unchanged. Initial missing callback, absent authorization claims and disallowed group scope were surfaced during the exercise; no first-user promotion, database role edit or admission bypass was used to recover access.

Synthetic regressions cover admin removal, removed admission, current refreshed ID tokens, unavailable/mismatched-subject UserInfo, invalid identity tokens and private account isolation. The operator explicitly deferred real admitted non-admin/denied accounts and provider-side group removal because no disposable secondary accounts exist. Those real-provider cases remain open.

## TMDB and AniDB

The existing TMDB token passed live movie/TV/person search, movie/TV/person details, English/Turkish movie translation, TV season metadata, movie and TV discovery, credits, provider lists and movie/TV country availability. Additional response checks verified translated title differences, TV original-language/minimum-vote constraints, populated cast/crew/related titles and source-supplied availability links. Desktop Chrome displayed live movie search/detail, Turkish-country offer categories, freshness and JustWatch attribution. No direct playback URL was fabricated.

The official AniDB title-only index was parsed and imported. One live native record was fetched through the database-backed cache with the registered public identity `watchledger`, version `1`. Its source identity, multilingual titles, regular and extra episode kinds and source artwork were present. The persisted request reservation was retained; no repeated live request was made to test throttling. Use remains the specification's personal non-commercial use, with source-specific licensing and attribution.

The agreed native-source limits remain: cached creator credits are available, but full creator biographies/filmographies, an exhaustive global upcoming catalog and a verified AniDB-tag/TMDB-genre equivalence are not supplied by the chosen integration. No additional source or guessed identity/genre merge was introduced.

## Mapping evidence and limits

The official animap release `v2026.10.21` was downloaded and checked against its published SHA-256, then imported through the real admin UI. Its checksum was `57d291192bd647657d093617d3164f07db87392f46d9bff3e5bb1d0b7fc4ca47`; source revision was `051ec3f7db309c7cc28bccd06535231ed4f13194`.

Resolving AniDB 1 produced zero approved relationships and 18 unresolved placements. The explicit TVDB episode identities did not yield unique TMDB episode matches; inferred defaults and absent explicit targets also remained unresolved. No episode-number fallback, title match or guessed relationship was accepted. This verifies safe failure for this source case, not broad successful split/combined coverage. Ordinary, split-cour, special and movie-alias cross-view acceptance still needs representative resolvable source identities and explicit relationship review. Synthetic projection/consent/correction/statistics regressions passed with PostgreSQL.

## Browser workflows

Chrome on this Mac exercised the following against disposable local data:

- Synthetic non-admin login, empty home state, live movie search/detail, library addition, rating and watched transition.
- Saved an actual portable export file; verified its synthetic rating/history and absence of roles/sessions.
- Created a custom status; previewed/applied a synthetic SIMKL conflict without overwriting the app's rating/watch decision.
- Switched interface language to Turkish while metadata language remained independent; inspected settings and the library at 390×844.
- Saved a separate current export for the real-login account, previewed and explicitly applied the synthetic portable export into that second identity, and verified the restored library.
- Created a completed-title smart list, verified the preview count and saved result contents.
- Verified the live TV season earlier-episode prompt defaults to only the selected episode; Escape cancels and the fixed implementation returns focus to the initiating button. Explicit save recorded only the selected episode with unknown date while the earlier episode remained unwatched.
- Inspected calendar controls, loaded statistics and credits. Statistics displayed one watched movie and its source runtime. The official TMDB logo and separate source notices were present.

This is real desktop/browser evidence. A narrow viewport is not a physical-phone or touch-device test. All screens in both languages, keyboard/dialog focus behavior, browser navigation races, manual drag ordering and installation/launch on the intended devices remain a broader acceptance matrix. Chrome exposed an install affordance, but a completed standalone install/launch was not established. No installed-app or phone acceptance is claimed. The earlier-episode dialog now has direct real-browser Escape/default/focus evidence; the rest of the broader dialog matrix remains separate.

## Optional SIMKL live access

The operator registered an AUTH V2 device/CLI client and approved the read-only PIN consent for this acceptance run. A separate temporary CLI acquired a token with exactly `media:read`; the production app did not create or refresh the grant. The access token and selected test-list ID were stored only in the existing ignored local environment, with values withheld from output. No refresh token was retained and no ongoing sync was added.

Authenticated user settings and the user's list index succeeded. The actual Go read-only adapter fetched the smallest non-empty list, validated the complete paginated response and saved the response privately. Its members supplied explicit external identities; the response contained no authentication tokens. No upstream write was performed.

The browser fetch step was paused because another Chrome extension popup blocked automation. It had not created a live API preview in the application database. Full browser preview/apply/repeat-import/cache-freshness acceptance therefore remains open. Synthetic tests still cover private cache isolation, premium/denied/expired errors, pagination consistency, app-edits-win and token-free archive/export behavior. A successful adapter fetch does not replace those remaining browser checks.

## Runtime, outages and recovery

Local Compose smoke passed with the production application image at `56a2327`, an isolated synthetic OIDC service and external disposable PostgreSQL. A test-only image layer supplied the synthetic issuer's public CA; TLS and hostname verification remained enabled. Startup/readiness, login, non-admin denial, restore preview-before-apply, known-date tracking, export, unavailable uncached metadata and tracking persistence after restart passed. The test-only provider and Compose containers were stopped afterward.

Real-issuer Compose startup did not pass. Issuer discovery from Docker returned connection refused, whereas discovery and HTTPS worked on the Mac host. Independent TCP probes in Docker bridge and host network modes also failed. No DNS, host routing, proxy, TLS verification or production configuration was changed to mask this difference. Real-provider container acceptance remains open until that network path works.

The operator explicitly authorized a disposable local Kubernetes cluster. A Docker-hosted k3s v1.35.5 cluster ran the rendered Helm chart with one ready replica, an existing synthetic Secret, external PostgreSQL and a synthetic OIDC service. Readiness, login, restore preview/apply, cached tracking and role-free export passed. The chart was from `56a2327`; its application image was the starting `9ad8405` production binary plus the public test CA. New OIDC options were separately verified on the host and by regression tests. This is chart runtime evidence, not real-provider Kubernetes or homelab acceptance.

Physical recovery was tested twice: dump a populated disposable database, restore into a separate empty database and compare complete history rows including date precision. The second recovery used the real Authelia/session configuration, followed by a fresh browser login and inspection of the restored completed movie/rating. All 17 migrations were present. Dumps and authentication/session material stayed private. Production backup infrastructure was not altered.

The synthetic runtime smoke verifies cached tracking with uncached metadata unavailable. Complete browser outage notices/freshness/retry acceptance remains open; an unconfigured test token is not evidence of every provider timeout/failure mode.

## Remaining inputs

- Complete real phone/touch/install acceptance and the remaining desktop workflow matrix.
- Supply representative explicit mappings that can be resolved and reviewed; do not infer replacements for the failed source case.
- Restore Docker reachability to the configured real issuer, then repeat real-provider Compose acceptance.
- Resume the optional live SIMKL browser preview/apply/reimport checks after the operator closes the blocking Chrome extension popup. Private acceptance credentials and a selected list are now available; the ordinary backup still contains no custom lists. No SIMKL token creation, refresh or server-side token persistence is implemented by the app.
- Keep real non-admin/denied/provider-side claim-removal checks explicitly deferred until the operator supplies disposable accounts.
