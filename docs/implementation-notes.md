# First-release implementation evidence

## Source verification, 2026-10-23

- Repository baseline passes `make lint test build` with Go 1.27.3 and Node 24.22.1.
- AniDB API overview, HTTP definition and policy return HTTP 403 from this environment. Current client registration, rate, cache and attribution requirements remain unverified. AniDB integration is gated; no fallback semantics have been substituted.
- animap release `v2026.10.21` downloaded outside the repository and its published SHA-256 verified. Schema is version 2, generated `2026-10-21T15:06:52Z`, mapping revision `051ec3f7db309c7cc28bccd06535231ed4f13194`, with 16,326 AniDB rows.
- The current animap schema maps TV episodes to TVDB, not TMDB TV. TVDB numbering must not be treated as TMDB numbering. TMDB external-ID resolution and explicit review remain necessary.
- Inspected ordinary, reordered/special and combined-film examples (AniDB 6098, 23 and 28). Explicit versus default-inferred targets are distinguished. Akira's six AniDB parts share TMDB movie 149; this is not permission to propagate progress without user consent.
- Dataset remains external. ODbL 1.0 / DbCL 1.0 attribution and provenance must accompany imported mappings; the application's MIT license does not cover that data.
- TMDB's official FAQ confirms noncommercial attribution requirements. Real TMDB requests require a deployment token; none has been supplied.
- Actual OIDC deployment claims and a private SIMKL export remain unavailable. Synthetic tests cannot establish the actual provider's refresh behavior or actual export coverage.

References: [animap release](https://github.com/cplieger/animap/releases/tag/v2026.10.21), [schema](https://github.com/cplieger/animap/blob/4681b20094a31dc7d00b52a1f3365e14ae8c6dbc/docs/schema.md), [license](https://github.com/cplieger/animap/blob/4681b20094a31dc7d00b52a1f3365e14ae8c6dbc/LICENSE-DATA), [TMDB FAQ](https://developer.themoviedb.org/docs/faq).

## Persistence and identity foundation

`make lint test build test-race` passes with disposable PostgreSQL 17.11. Migration tests exercise initial application, repeat application, and rejection of a cross-account status foreign key. Current-claim tests cover missing admission, separate admin grants, admin removal at refresh, and mismatched UserInfo subjects. Session ciphertext tampering is rejected. Full browser login and actual deployment-provider compatibility remain unverified; related checklist items remain unchecked.
