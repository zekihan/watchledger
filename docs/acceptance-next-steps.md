# Next acceptance steps

Reviewed locally on 2026-10-24, starting from `41190d7`. See [the latest acceptance evidence](acceptance-2026-10-24.md) for completed provider, browser and runtime checks. Implementation has merged, but first-release acceptance remains incomplete. Keep checks tied to their tested revision. The outstanding checklist is [TODO.md](../TODO.md).

## Work available without operator credentials

The official TMDB logo was retrieved through Chrome and committed in `d118b5b`; its source and unchanged checksum are in [data sources](data-sources.md#tmdb-logo). `make lint test build` passed using Go 1.27.3 and Node 24.22.1. Database-dependent tests were skipped because no disposable test database was configured; this is not a database or live-provider acceptance result.

The Helm chart passed `helm lint` and `helm template` locally with synthetic values for `publicURL`, `oidc.issuer` and `existingSecret`. Empty defaults deliberately fail required-value validation. Rendering does not prove Kubernetes startup. No cluster was selected or changed.

Chrome and Docker are available on this Mac. Earlier cloud-browser limitations do not establish a blocker here. Authenticated browser acceptance still needs a disposable application database and test OIDC setup. No real-provider or authenticated browser acceptance is claimed by this review.

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

## Operator steps in order

1. Choose an OIDC provider and create a disposable client. Register the exact loopback callback `http://127.0.0.1:5173/auth/callback` for the Vite setup, or `http://127.0.0.1:8080/auth/callback` for the built app. Keep the matching `PUBLIC_URL`. Prepare admitted non-admin, admitted admin and denied test accounts. Configure separate admission/admin claims; verify refresh or matching-subject UserInfo returns current claims. Follow [development](development.md) and [authorization recovery](deployment.md#authorization-configuration-recovery). Keep secrets in a local environment, never chat or Git.
2. Prepare a disposable PostgreSQL 17+ database and local configuration from `.env.example`. Supply a persistent session key for that disposable installation. Export environment variables explicitly; `.env` is not loaded automatically. Run the [local acceptance setup](local-acceptance.md#setup-and-credentials), including database tests with the disposable `TEST_DATABASE_URL`.
3. Supply the existing TMDB read-access token locally. Test movie/TV/person metadata and availability. The logo is handled; live source behavior remains open. Confirm the intended AniDB use complies with its policy before live native-source checks.
4. Use synthetic history first. Run desktop Chrome acceptance in English and Turkish; test phone-width layouts separately from installation on a physical phone. Exercise keyboard focus, dialogs, tracking, lists, import cancellation and cached-provider outages. Record screenshots without credentials or private history.
5. Save an actual portable export before any private import. Preview the supplied backup, inspect every unresolved identity and summary warning, explicitly apply, verify archive preservation, then test repeat import and app-edits-win behavior. Custom-list migration needs the separate input described above. Do not overwrite the original backup.
6. Verify two-account isolation and export/restore in disposable accounts. Exercise ordinary/split-cour/special/split-combined/movie-alias mapping coverage with explicit IDs and consent. These checks cannot be replaced by backup structure inspection.
7. Verify real Compose configuration and backup recovery locally. Select and explicitly authorize a disposable Kubernetes target only if Helm runtime acceptance is desired. Existing homelab deployment is outside this checklist's authorization.

The OIDC client is now configured: real admin login and forced refresh passed with the group scope and explicit subject-validated UserInfo. The operator deferred real non-admin/denied-account checks. Helm runtime and local database recovery passed. Remaining work is full device/browser acceptance, broader explicit mapping coverage, Docker-to-real-issuer reachability and the remaining live SIMKL browser workflow. The operator-approved read-only PIN grant and actual list adapter fetch now pass; Chrome’s extension popup must be closed before resuming the UI check. For the live-list acceptance run only, local `SIMKL_CLIENT_ID`, `SIMKL_ACCESS_TOKEN` and `SIMKL_LIST_ID` can supply the existing AUTH V2 credentials and selected list; the application itself reads these through its temporary import form, not environment variables. Do not create a permanent server-side token configuration or print credentials.
