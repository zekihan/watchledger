# Synthetic Compose smoke fixture

This override is for disposable GitHub CI runners only, not a deployment option for real users. Run it through `scripts/compose-smoke.sh` after building `watchledger:ci`. The script refuses a non-GitHub environment and uses a unique CI project, a fresh PostgreSQL container and an ephemeral synthetic OIDC issuer. It never reads an operator database URL or provider credentials.

The fixture generates a short-lived test certificate, explicitly trusts that certificate only in the test app/client, and verifies TLS hostnames. It does not disable certificate verification or change host trust. Its fake login requires the app's PKCE/nonce flow and yields only a synthetic non-admin account. The fixture executable is not part of the production app entrypoint.

The checks cover the actual Compose definition, migrations/readiness, static manifest, login, non-admin denial, preview-before-apply restore, known-date movie tracking, role-free portable export, unavailable uncached metadata, and persistence across app restart. Containers and fixture files are removed afterward. This does not validate your real identity provider, metadata credentials, browser/device layout, or Kubernetes runtime.
