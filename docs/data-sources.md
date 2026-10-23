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

The official API and policy pages returned access errors during the design investigation. Exact current HTTP API client, rate, and cache requirements were not verified. Do not hardcode remembered limits as though they were checked facts.

Verification targets:

- [API overview](https://wiki.anidb.net/API).
- [HTTP API definition](https://wiki.anidb.net/HTTP_API_Definition).
- [Site policy](https://anidb.net/policy).

Integration is gated on verifying these requirements. If the source is unsuitable, review the Kitsu fallback and its differences before changing native-entry behavior.

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

Verified against the official [custom-lists guide](https://api.simkl.org/guides/custom-lists) and [standard media objects](https://api.simkl.org/conventions/standard-media-objects). The documented list-contents response is separate from the downloadable watchlist backup. The read-only beta API requires AUTH V2 and PRO/VIP access; HTTP 200 can contain a premium-only error rather than list data. Watchledger does not connect an account, request credentials, scrape lists, or call this API.

The file importer accepts one complete contents response or all unmodified pages for one list. Multi-page files must agree on source identity, update marker, totals, media type and sort. Incomplete/error/index responses fail; response array order is preserved rather than interpreting position or echoed sort fields. Automatic lists become explicitly labelled manual snapshots; original rules and unsupported fields remain in the private archive.

Members resolve only through explicit provider identities or this user's previously previewed SIMKL identities, and must already exist in the private library. Conflicting or collapsed identities block the whole list for review. Anime never resolves via shared TMDB IDs. Existing app list edits and deletions win on reimport and portable restore. This does not establish custom-list coverage inside ordinary SimklBackup.json or live API acceptance.
