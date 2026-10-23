# Next acceptance steps

Reviewed locally on 2026-10-24, starting from `41190d7`. See [the latest acceptance evidence](acceptance-2026-10-24.md) for completed provider, browser and runtime checks. Implementation has merged, but first-release acceptance remains incomplete. Keep checks tied to their tested revision. The outstanding checklist is [TODO.md](../TODO.md).

## Current local results

The [dated acceptance record](acceptance-2026-10-24.md) supersedes the initial unconfigured review. A disposable PostgreSQL installation, real Authelia admin login/refresh/configuration recovery, live TMDB and AniDB reads, local Helm runtime smoke, database restore and actual browser import/export checks have passed. Database-backed tests are enabled. The logo is committed. Real-provider Compose remains blocked specifically by Docker’s route to the private issuer, while host access works.

English and Turkish populated main screens fit verified Chrome phone viewports. The operator selected Android Chrome and requested Chrome phone-size checks; physical Android touch, installation and authentication return are deferred until a tested HTTPS deployment. Viewport checks do not establish those device behaviors. Local desktop Chrome installation, opening a standalone authenticated window and real OIDC sign-out/sign-in return to Home have passed; quitting and relaunching the installed app also returned to Home at `/` with the session preserved.

## Observed SIMKL backup coverage

The two JSON files supplied locally were byte-identical. A temporary, uncommitted test exercised `simkl.Parse` at `d118b5b` against the private backup and reported no record-level issues. Every record had a supported explicit title target. This does not verify provider metadata or episode equivalence. No source title, source ID, personal date, rating, viewing count, original file or normalized history is included here.

| Area             | Observed fields and behavior                                                                | Remaining validation                                                                       |
| ---------------- | ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Sections         | `movies`, `shows`, `anime` arrays                                                           | Preview and apply through the UI                                                           |
| Status           | All five supported SIMKL status values occur                                                | Verify resulting library statuses                                                          |
| Identity         | Movie/show TMDB IDs and native AniDB IDs are present                                        | Load exact metadata; resolve episodes explicitly                                           |
| Progress         | `seasons[].number`, `episodes[].number`; explicit gaps occur                                | Never infer episode identity from equal numbers                                            |
| Dates            | Episode `watched_at` and movie `last_watched_at` timestamps; watchlist timestamps           | Compare exported date precision after apply                                                |
| Ratings          | `user_rating` is present but null throughout                                                | Non-null rating handling remains synthetic-test coverage                                   |
| Summary counters | Some counters differ from explicit history; a completed native entry has no episode records | Retain warnings and inspect review; do not fill gaps from counters                         |
| Custom lists     | No custom-list section                                                                      | Supply a complete separate list-response file or use optional existing eligible API access |
| Other fields     | Rewatch, rating-time, next/last-episode summaries and auxiliary IDs                         | Preserve original archive; do not reinterpret unsupported fields                           |

This deliberately synthetic example illustrates the observed field structure and a gap; its values are invented:

```json
{
  "shows": [
    {
      "show": {
        "title": "Synthetic example",
        "ids": { "simkl": 100, "tmdb": 200 }
      },
      "status": "watching",
      "user_rating": null,
      "watched_episodes_count": 2,
      "seasons": [
        {
          "number": 1,
          "episodes": [
            { "number": 1, "watched_at": "2026-01-02T12:00:00Z" },
            { "number": 3, "watched_at": "2026-01-04T12:00:00Z" }
          ]
        }
      ]
    }
  ],
  "movies": [],
  "anime": []
}
```

Episode 2 must remain unwatched. These source positions must enter explicit episode resolution rather than automatically selecting matching provider numbers. Structural parsing alone does not establish migration success.

## Remaining steps in order

1. Close the Chrome extension popup if it blocks the acceptance tab. The remaining browser checks can then continue locally with the existing configuration; no credentials need to be pasted into chat. The live SIMKL grant is read-only and temporary. Actual three-page adapter coverage, one-list UI preview/apply/repeat, token clearing, original cache timestamps and preservation of renamed/reordered app edits have passed. The live adapter and Chrome UI also rejected a synthetic invalid token without a preview or credential disclosure; account-isolated cache access and remaining import/navigation races are still separate checks.
2. Resolve Docker-to-private-issuer reachability. Both environments resolve the same address; Docker reaches public HTTPS and the Mac app, and the issuer address does not overlap Docker bridge networks. The operator confirmed **System Settings → Privacy & Security → Local Network → Docker** is enabled. Inspect the Docker backend’s private-network/VPN/firewall path with the appropriate host administrator; the cause is not yet established. Do not disable TLS or change issuer identity. Docker describes [backend networking](https://docs.docker.com/desktop/features/networking/) and the [Local Network permission](https://docs.docker.com/desktop/troubleshoot-and-support/troubleshoot/topics/#containers-cant-reach-devices-on-my-local-network). Repeat real-provider Compose startup and login only after transport works; other local containers must not be restarted as an incidental test.
3. Extend mapping coverage using the verified release and actual source identities. The reviewed AniDB 123 / TMDB 125513 movie alias passed cross-view progress, rating/unknown-date preservation, no duplicate library item and connected-removal cancellation. Frieren (AniDB 17617) has 35 explicit candidate placements independently verified against native metadata, TMDB Find and actual season records, with 56 placements still unresolved. The Frieren proposal, ordinary/special tracking and correction/retained-event removal have now passed in Chrome. New Panty & Stocking also passed split/combined consent and partial-progress checks; the two 86 cours passed shared progress with independent native ratings/statuses. Native-unit statistics, separate partial/source-part counts and unknown-date monthly exclusion also passed; keep the mixed full-film/part Zoku Owarimonogatari proposal unsaved until edition semantics are explicitly resolved. Unresolved IDs must remain unresolved rather than being matched by number or title.
4. After an independently authorized deployment of the tested revision over HTTPS, test Android Chrome. Open the deployed origin, sign in, confirm the private library and source/date/rating values, install through Chrome or the app’s browser-provided install action, and launch the installed icon. Verify the Watchledger name/icon, standalone launch at the expected origin and successful return after sign-out/sign-in. Tap tracking controls and cancel an earlier-episode prompt once before saving a synthetic unknown-date event. Repeat key screens in Turkish. Record Chrome/Android versions and any clipped controls. This step does not authorize deployment, LAN exposure or changes to production history.
5. Keep real admitted non-admin, denied-user and provider-side group-removal cases deferred as requested. When disposable accounts exist, verify non-admin restrictions, denied admission, removal of admin/admission at refresh and matching-subject current claims. The local server-side configuration recovery exercise already passed without a database role bypass.
6. Preserve current exports before any additional import. The supplied ordinary backup has no custom-list section; its structural coverage above does not establish migration of all private history. Continue explicit episode resolution, app-edits-win, unknown-date preservation, private source-archive checks and portable restore only in disposable accounts until the full matrix passes.

For optional live-list acceptance only, local `SIMKL_CLIENT_ID`, `SIMKL_ACCESS_TOKEN` and `SIMKL_LIST_ID` supply existing AUTH V2 inputs to the temporary form. The application does not read these environment variables or create/refresh grants. Do not turn the acceptance token into permanent server configuration or print credentials. Keep uncompleted checks open in [TODO.md](../TODO.md), and keep implementation fixes in [the feature tracker](features.md).
