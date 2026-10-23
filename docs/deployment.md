# Deployment and recovery

The first release is still under implementation. Do not treat packaging or a healthy process as acceptance of all product workflows. No user infrastructure is modified by development checks.

## Configuration

Build the application image with `docker build -t watchledger:0.1.0-dev .`. Both frontend and API run in one container. PostgreSQL 17+ is external. Export the variables documented in `.env.example`; Compose loads an explicitly provided environment file with `docker compose --env-file /private/path/watchledger.env -f deploy/compose.yaml up -d --build`.

The image runs as UID/GID 65532, without root capabilities. Put TLS at your existing reverse proxy and set `PUBLIC_URL` to the exact external origin. Cookie security and write-origin checks use it. Do not place provider credentials in frontend variables or build arguments.

For Helm, supply `existingSecret`, `publicURL`, and `oidc.issuer`. The separately provisioned Secret contains `DATABASE_URL`, `SESSION_KEY`, `OIDC_CLIENT_SECRET` when needed, and `TMDB_TOKEN`. Values do not create secrets. Use an existing ingress/reverse proxy; the chart creates only a Deployment and Service. One replica is required while login transactions remain in process memory. There is no first-user admin bootstrap.

Example template validation with synthetic configuration:

```sh
helm lint deploy/helm/watchledger --set existingSecret=fixture --set publicURL=https://watch.example --set oidc.issuer=https://identity.example
helm template watchledger deploy/helm/watchledger --set existingSecret=fixture --set publicURL=https://watch.example --set oidc.issuer=https://identity.example
```

`/healthz` is liveness. `/readyz` verifies the database is reachable. Startup performs checksum-verified migrations under a database lock and fails if migration or OIDC discovery fails. Restore provider access before restarting during a provider outage; already running cached tracking does not require catalog access.

## Backup and restore

Per-user export does not contain authentication sessions, server secrets, or admin authority. Keep downloaded backups private. The restore preview lists existing library records as conflicts; existing app records win, and explicit unwatch tombstones prevent resurrection. Export a current backup before applying a restore. Keep the session encryption key in your infrastructure secret backup, separately from portable user data.

Instance recovery is an operator task. Before upgrades, create a consistent custom-format PostgreSQL backup with `pg_dump --format=custom --file=watchledger.dump "$DATABASE_URL"`. Test recovery into a separate empty database using `pg_restore --no-owner --dbname="$RECOVERY_DATABASE_URL" watchledger.dump`. Never point test restoration at the running instance. Preserve database roles/ownership through your normal infrastructure process. Verify schema versions, account isolation, history counts and date precision, and then perform an authorized login and read-only library check. Changing a session key intentionally invalidates stored OAuth tokens.

Synthetic Compose runtime smoke passed in [CI run 39997022951](https://github.com/zekihan/watchledger/actions/runs/39997022951): actual startup/migrations, OIDC login, restore preview/apply, tracking/export and persistence after restart. It uses the production image and Compose definition with a CI-only isolated database/provider override; see `deploy/smoke/README.md`. This does not establish compatibility with your real provider configuration. Helm rendering/lint passes, but an actual Kubernetes deployment remains an operator-selected disposable-environment check. Do not deploy to the homelab as a substitute.

## Authorization configuration recovery

There is no emergency local login, first-user promotion, or database-admin role override. If an intended account is denied, use your existing server and identity-provider administration access to diagnose the configured issuer/client, callback origin, admission claim/value and separate admin claim/value. Correct the deployment configuration or the intended provider claims; do not relax admission to admit everyone or edit `users.admin` to manufacture authority. Keep the issuer/subject binding stable: changing issuer or subject creates a different identity and is not an account-recovery shortcut.

After an authorized configuration correction, restart the app with the corrected settings and sign in again. To require existing application sessions to obtain fresh claims immediately rather than waiting up to an hour, an authorized operator can expire their cached authorization evaluation with `UPDATE sessions SET expires_at=now();` against the correct application database. This does not grant roles, change history, or revoke upstream provider credentials. The next request must successfully refresh and satisfy the current claim policy, otherwise it requires a new login. Preserve the session encryption key; replacing it invalidates stored tokens and is not needed merely to fix a claim path.

Verify an admitted non-admin account, an intended admin account, and a denied account after recovery. Missing current admission still denies access; missing current admin removes admin privileges. If the identity provider cannot supply current claims in a validated refreshed ID token or matching-subject UserInfo, fix that provider integration rather than reusing old claims. These operational changes are not executed by the development workflow; deployment-specific verification is in TODO.md.
