# Local acceptance checklist

The merged implementation is not an accepted first release. [TODO.md](../TODO.md) collects the checks and assets needing operator involvement. Run these checks on a disposable local installation before using real history. Source-dependent limitations in `implementation-notes.md` are not waived by this checklist.

## Setup and credentials

1. Use the revision being accepted and record its commit ID. Use the Go/Node versions pinned in the repository.
2. Follow `development.md` and `deployment.md` to configure a disposable PostgreSQL database and an OIDC test client with an exact callback URL. Keep admission and admin claims separate.
3. Keep credentials in your local environment or a git-ignored local `.env` file. The binary does not automatically load `.env`. Never commit credentials or include them in logs/screenshots.
4. Configure the TMDB API Read Access Token as `TMDB_TOKEN`, not the shorter API key. Confirm that your terminal can reach the official API before diagnosing authentication.
5. Run `npm ci`, then `make lint test build test-race`. Set `TEST_DATABASE_URL` only to a disposable database; without it, database tests are skipped. Unit fixtures are synthetic and do not establish live-provider compatibility.

## Live provider and authentication checks

- Log in with an admitted non-admin account and confirm that admin tools remain unavailable.
- Test denied admission and a separate admin account. Verify refresh reevaluates current claims rather than keeping removed access/admin grants.
- Search a known movie, TV series, and person; open details, seasons, credits, and viewing offers for your country. Check requested-language fallback, source attribution, and explicit unavailable/error states.
- Disconnect provider access after caching a title. Cached tracking must remain usable; uncached queries must report failure rather than fabricated data.

## Private tracking and portability

- Use two accounts to verify that libraries, statuses, lists, history, exports, and import archives remain private.
- Mark a single episode after a gap. Confirm that only that episode is selected by default and that the earlier-episode choice is explicit.
- Try date-only, timestamp, and unknown watch dates. Repeat bulk actions and ensure known dates remain intact.
- Test custom-status locking, rename, replacement, smart-list dependencies/cycle rejection, and negative filters against unknown availability.
- Export before importing. Save the file yourself; an export request alone does not establish a recoverable backup.
- Use a small synthetic SIMKL JSON first. Preview must not apply history. Application requires a fresh export and an explicit action. Repeat the same import and verify no duplicated history.
- Edit a rating or explicitly unwatch in the app, then reimport. Existing app decisions must win. Download the source archive and verify unsupported fields remain present.
- Open an uncached review, load its explicit source catalog ID, and choose the provider seasons to load. Loading metadata alone must not add private history. Native anime or missing source IDs must not be guessed.
- Open an unresolved episode review and select each actual cached target episode. Equal numbers must not auto-select targets. Confirm the choices explicitly; duplicate/foreign targets and changes to a previously confirmed source identity must be rejected. Test an unwatch followed by reimport to ensure it stays unwatched.
- Native metadata and confirmed alternate tracking are implemented, but broad split/combined import resolution and actual backup custom-list coverage still require verification. Do not treat unresolved items as a complete migration of your real library.
- Export/restore into a disposable second account. Inspect conflicts, unknown dates, list references, and settings. Authorization and sessions must never be restored from portable data.

## Browser and packaging acceptance

- Check English/Turkish desktop and phone layouts, keyboard focus, loading/error/empty states, and the tracking/import dialogs.
- Retry, cancel, navigate back, and upload another file while a preview is loading; stale results must not become the active import.
- Verify Compose startup with real test configuration. Container build success is not a startup test.
- Render and validate Helm, then perform a disposable deployment only in an environment you explicitly choose. No homelab deployment is part of the development task.
- Do not mark anime mapping/consent, source-policy verification, branding, or unresolved import-resolution requirements complete from these smoke checks alone.

## Separate custom-list response files

- Use a synthetic complete SIMKL list-contents response first, not a list index or premium-only error. Upload all pages together; incomplete or inconsistent snapshots must fail.
- Ensure every member already exists in your private library with an explicit supported identity. An unresolved member must queue the whole list, not silently create a partial list.
- Export, preview, and explicitly apply. Check response order, manual snapshot labelling for auto lists, and preservation of unsupported fields in the downloadable source archive.
- Rename or reorder an imported list, remove a member, and reimport. App edits must remain. Delete an imported list and reimport; it must stay deleted.
- Export/restore into a disposable second account and repeat reimport. Source identity must prevent duplicates; deletion markers must prevent resurrection. No status, rating or watch date should change from this list-only import.
- This file-upload workflow does not fetch the SIMKL API or establish paid-access eligibility. The separate optional live fetch is described below. Do not put credentials in uploaded files.

## Native anime and alternate tracking

- Configure your registered AniDB client name/version. The provided registration is `watchledger`, version `1`; it is a public client identifier, not an account password. Check the deployment's permitted use against the source policy.
- Upload the official title-only index in the admin screen. Search an alias, explicitly load a native title, and check native episode types, artwork fallback, title preferences, and cached access. Do not repeatedly refresh AniDB to test rate limits against the live service.
- Upload a versioned animap release with its checksum sidecar. Load required provider metadata, resolve explicit identities, and review proposed episode relationships. Unresolved/inferred/movie placements must not become guessed episode links.
- Save the native entries for separate cours. Check their individual statuses/ratings in the aggregate show view; do not expect a synthetic combined rating.
- Review split/combined links by title and episode. Before confirmation, cross-view progress must remain blocked. Cancel once, then explicitly confirm the current revision.
- Track one alternate show episode without adding that show as another library item. Verify the native progress/status changes, its rating stays intact, and no duplicate private library entry appears. Repeat with season/whole scopes and preserve known dates.
- Unwatch a derived episode: the connected-record removal dialog must default to cancel. Accept only after inspecting the affected source records. A corrected/stale plan must fail and require another preview.
- Change a mapping after recording alternate history. The event must remain visible, its original date must survive, and removing your own retained event must still work. New watching through an unconfirmed link must remain blocked.
- Export and restore this synthetic history to a second account. Alternate history containers must survive without restoring mapping consent or manufacturing aggregate ratings/statuses. Repeat restore and check that records do not duplicate.
- Compare Up next in native/show modes, move cards with both drag and visible buttons, save unrelated settings, and verify ordering remains. Unmapped or unaired counterparts must fall back visibly rather than advertise a guessed next episode.

## Optional live SIMKL custom-list import

- Use an existing AUTH V2 client ID and a PRO/VIP token belonging to your test account. Enter the temporary token only in the application's password field over HTTPS or loopback, never in chat, URLs, source files, logs or uploaded backups. Do not configure request-body logging at a reverse proxy.
- Fetch one known list ID. Confirm that the field clears immediately, all pages produce a preview, and no list/history/rating changes before explicit application. The app does not obtain new grants or perform ongoing sync.
- Verify premium-only, wrong-generation token, denied access, expired token and rate-limit errors are meaningful and do not display the token. Never use production credentials in recorded screenshots.
- Fetch an auto list again within a day and verify its original freshness timestamp remains. Confirm another application account cannot read that private cached response.
- Choose another file or leave the screen during a fetch. An old response must not become the active preview.
- Download your current export, apply explicitly, and repeat import. The existing source identity and app-edits-win rules must prevent duplicate memberships or unwanted rewrites. Inspect exports and the source archive to confirm they contain list data but no access token.
