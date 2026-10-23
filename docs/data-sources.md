# Data sources

Source review date: 2026-10-23. Recheck terms, access rules, and endpoint behavior before implementation. This document records the source investigation behind the specification, not a guarantee of complete data coverage.

## TMDB

Planned use: general movie/TV/person metadata, episodes, discovery, and country-specific provider availability.

- [API FAQ](https://developer.themoviedb.org/docs/faq): noncommercial use with attribution; commercial licensing is separate.
- [TV watch providers](https://developer.themoviedb.org/reference/tv-series-watch-providers): JustWatch-powered subscription/rental/purchase availability by country; JustWatch attribution required; full direct playback links are not supplied.
- [Movie discovery](https://developer.themoviedb.org/reference/discover-movie): cast/crew/people, runtime, original language, vote score/count, and availability filters.
- [TV discovery](https://developer.themoviedb.org/reference/discover-tv): does not document the equivalent cast/crew filter surface.
- [Person TV credits](https://developer.themoviedb.org/reference/person-tv-credits): candidate source for TV person-based discovery, followed by local filtering/enrichment.
- [TV external IDs](https://developer.themoviedb.org/reference/tv-series-external-ids) and [episode groups](https://developer.themoviedb.org/reference/tv-episode-groups): not a guarantee of complete anime-native identity or episode mapping.

Keep community scores distinct from personal ratings. Do not infer available audio/subtitle languages from original production language.

## AniDB

Planned use: native anime identity, metadata, and episode organization alongside TMDB's alternate structure.

The operator supplied saved official documentation on 2026-10-23 after direct access returned 403. HTTP API revision 24120 requires a registered lower-case HTTP client identifier and registered version, UTF8/gzip handling, no more than one request every two seconds, and heavy caching without repeated dataset requests within a day. HTTP 200 may contain an XML error. The API overview (revision 17954) permits a title-only search index updated at most once per day; this is not permission to mirror the full metadata database.

The supplied policy states CC-BY-NC-SA 4.0 for AniDB content, prohibits commercial/revenue-generating use and scraping/bulk database extraction, and distinguishes artwork rights. Keep data attribution/license notices separate from the application MIT license. Client registration and live integration acceptance remain operator prerequisites. No account or credential was created during development.

Verification targets:

- [API overview](https://wiki.anidb.net/API).
- [HTTP API definition](https://wiki.anidb.net/HTTP_API_Definition).
- [Site policy](https://anidb.net/policy).

Implementation can now follow the verified requirements; live access remains disabled until a registered client is configured. If the source is unsuitable for a deployment, review the Kitsu fallback and its differences before changing native-entry behavior.

## animap

Planned use: mappings between native anime identities/episodes and TMDB/TVDB placement, combined with explicit local admin overrides.

- [Project documentation](https://github.com/cplieger/animap).
- [Data license](https://github.com/cplieger/animap/blob/main/LICENSE-DATA): ODbL 1.0 and DbCL 1.0; distinct from this repository's MIT license.

The dataset distinguishes written mappings from inferred defaults. Mapping coverage and correctness are not guaranteed. Retain provenance and version information, preserve user history through updates, and route ambiguity to review. Do not bundle the dataset into this repository without evaluating its distribution requirements.

## SIMKL

Planned use: user-initiated migration, starting with JSON exports and using read-only API access only if needed.

- [Backup options](https://simkl.com/apps/backup/): CSV does not preserve all episode gaps; inspect JSON before assuming its field coverage.
- [Sync guide](https://api.simkl.org/guides/sync): episode arrays, watched timestamps, statuses, ratings, and original versus synthesized completed-show episode records.
- [Anime guide](https://api.simkl.org/guides/anime): split cours, absolute numbering, shared TMDB IDs, and TVDB/TMDB boundary differences.
- [Rewatch guide](https://api.simkl.org/guides/rewatches): possible source data retained in archives, while first-release rewatch UI is deferred.
- [API rules](https://api.simkl.org/api-rules): do not assume unrestricted catalog use for a competing tracker. Ongoing SIMKL metadata ingestion is not the selected design.

Some API export options can synthesize missing episode rows for completed shows using a last-watched date. Preserve factual dates only and label inferred completion with unknown dates. A shared TMDB ID is not sufficient evidence to merge anime entries.

## Fallbacks and excluded assumptions

- [Kitsu API documentation](https://kitsu.docs.apiary.io/) documents public anime/episode GET endpoints. It is a potential fallback, not a promise of AniDB/SIMKL-equivalent entry boundaries or blanket data licensing.
- [AniList API terms](https://docs.anilist.co/guide/terms-of-use) include restrictions on competing non-complementary tracker services. It is not the selected source; free access is not permission to ignore those restrictions.
- No provider establishes complete, permanently correct bidirectional anime mappings.
- The initial design review used public documentation only. During implementation, an operator-provided private JSON backup was inspected locally to verify its schema. It is not included in the repository; committed fixtures remain synthetic.

## OIDC reference

- [OIDC refresh response](https://openid.net/specs/openid-connect-core-1_0.html#RefreshTokenResponse): a successful refresh can omit a new ID token.
- [UserInfo](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo): usable for current claims when configured; validate the response subject against the authenticated identity.

Provider support for refreshed custom claims must be checked against the actual deployment. The app must not extend old authorization simply because token refresh succeeded.

## SIMKL custom-list response files

Verified against the official [custom-lists guide](https://api.simkl.org/guides/custom-lists) and [standard media objects](https://api.simkl.org/conventions/standard-media-objects). The documented list-contents response is separate from the downloadable watchlist backup. The read-only beta API requires AUTH V2 and PRO/VIP access; HTTP 200 can contain a premium-only error rather than list data. The file-import path does not connect an account, request credentials, scrape lists, or call the API. The optional read-only fetch path is documented below.

The file importer accepts one complete contents response or all unmodified pages for one list. Multi-page files must agree on source identity, update marker, totals, media type and sort. Incomplete/error/index responses fail; response array order is preserved rather than interpreting position or echoed sort fields. Automatic lists become explicitly labelled manual snapshots; original rules and unsupported fields remain in the private archive.

Members resolve only through explicit provider identities or this user's previously previewed SIMKL identities, and must already exist in the private library. Conflicting or collapsed identities block the whole list for review. Anime never resolves via shared TMDB IDs. Existing app list edits and deletions win on reimport and portable restore. This does not establish custom-list coverage inside ordinary SimklBackup.json or live API acceptance.

## Versioned mapping ingestion and external-ID resolution

The mapping reader is pinned to mapread v0.2.0. The v2026.10.21 animap release was checksum-verified and decoded during implementation (SHA-256 57d291192bd647657d093617d3164f07db87392f46d9bff3e5bb1d0b7fc4ca47). Current version-2 source metadata must contain the mapping revision; incompatible earlier version-2 shapes are rejected by the upstream reader. No dataset is bundled in this repository.

[TMDB Find By ID](https://developer.themoviedb.org/reference/find-by-id) explicitly supports TheTVDB episode identifiers. The bridge uses those identities, then checks the returned TMDB episode against cached provider season metadata. It never equates TVDB and TMDB season/episode positions. Missing IDs, inferred defaults and ambiguous provider responses remain explicit review items rather than guessed links. Movie identities use a separate proposal kind: the official animap schema defines multiple TMDB movie IDs as aliases of one work, not different parts. Movie proposals require exact cached native episode and movie identities plus explicit alias confirmation.

Dataset uploads retain immutable source bytes, checksum, release version, source revision, generation timestamp and original attribution. Imported data remains separately licensed under ODbL/DbCL and the stated upstream notices; it is not relicensed as MIT. Admin review produces a revision-bound proposal; uploading a dataset alone neither changes approved mappings nor propagates private watched flags.

## Optional read-only SIMKL API list fetch

The current custom-list guide requires an AUTH V2 client ID and a PRO/VIP user access token. The app can retrieve one explicitly selected list with GET requests to `https://api.simkl.com/lists/{id}`, sequentially paginate, and then reuse the existing private preview/apply workflow. It does not request new OAuth grants, refresh/store tokens, discover or mirror the catalog, write to SIMKL, or perform ongoing synchronization. Operators/users must supply an already obtained AUTH V2 token in the application, never in chat or committed configuration.

The token is sent only in the upstream Authorization header and is not placed in URLs, persisted imports, exports, or diagnostic errors. Redirects are refused. API errors, including HTTP-200 premium-only responses, changing/incomplete snapshots, excessive pagination and oversized bodies fail before preview. Auto-list fetch results are cached privately for 24 hours; other explicit fetches for five minutes, without extending freshness on cache reads. Source item links retain numeric SIMKL IDs and available slugs. The official rules and custom-list guide are linked in the interface. Live access eligibility and an actual V2 grant remain operator acceptance checks.

## TMDB logo

The credits page uses the unmodified official Alt short (blue) SVG, retrieved from the [TMDB logo gallery](https://www.themoviedb.org/about/logos-attribution) through Chrome on 2026-10-23. Its source is [the official asset](https://www.themoviedb.org/assets/v4/logos/v2/blue_short-8e7b30f73a4020692ccca9c88bafe5dcb6f8a62a4c6bc55cd9ba82bb2cd95f6c.svg). The local file is `web/public/credits/tmdb.svg`; SHA-256 is `8e7b30f73a4020692ccca9c88bafe5dcb6f8a62a4c6bc55cd9ba82bb2cd95f6c`. Formatting excludes this third-party file to preserve its bytes.

The logo remains TMDB's asset under its own branding rules, outside the application's MIT license. Keep its colors and aspect ratio unchanged, display it less prominently than Watchledger branding, and retain the non-endorsement notice in the credits section as required by the [official FAQ](https://developer.themoviedb.org/docs/faq).
