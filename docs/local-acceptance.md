# Local acceptance checklist

The feature branch is not a completed first release. Run these checks on a disposable local installation before using real history. Source-dependent limitations in `implementation-notes.md` are not waived by this checklist.

## Setup and credentials

1. Check out `feat/first-release` and use the Go/Node versions pinned in the repository.
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
- Open an unresolved episode review and select each actual cached target episode. Equal numbers must not auto-select targets. Confirm the choices explicitly; duplicate/foreign targets and changes to a previously confirmed source identity must be rejected. Test an unwatch followed by reimport to ensure it stays unwatched.
- Split/combined episode mappings, native anime integration, and custom-list migration are still incomplete. Do not use this partial importer as a complete migration of your real library.
- Export/restore into a disposable second account. Inspect conflicts, unknown dates, list references, and settings. Authorization and sessions must never be restored from portable data.

## Browser and packaging acceptance

- Check English/Turkish desktop and phone layouts, keyboard focus, loading/error/empty states, and the tracking/import dialogs.
- Retry, cancel, navigate back, and upload another file while a preview is loading; stale results must not become the active import.
- Verify Compose startup with real test configuration. Container build success is not a startup test.
- Render and validate Helm, then perform a disposable deployment only in an environment you explicitly choose. No homelab deployment is part of the development task.
- Do not mark anime mapping/consent, source-policy verification, branding, or unresolved import-resolution requirements complete from these smoke checks alone.
