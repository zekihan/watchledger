# Deployment packaging

Version 0.1.3 publishes Linux amd64/arm64 images at `ghcr.io/zekihan/watchledger:0.1.3` and `docker.io/zekihan/watchledger:0.1.3`, and chart `oci://ghcr.io/zekihan/charts/watchledger:0.1.3`.

The application container, `compose.yaml` and `helm/watchledger` chart are implemented. The container serves the built React frontend and Go API, performs PostgreSQL migrations, authenticates with OIDC and runs the bounded refresh queue. PostgreSQL and the identity provider are configured externally; no local-password or first-user admin fallback is provided.

Follow the [deployment and recovery guide](../docs/deployment.md) for environment variables, existing Secrets, external PostgreSQL, TLS termination and backup/restore. The chart runs one replica while OIDC login transactions remain in process memory.

CI checks image builds, chart rendering and a synthetic Compose runtime/login/restart smoke. These do not establish configured-provider compatibility, real-device behavior or a Kubernetes deployment. The remaining operator-owned checks and source assets are in [TODO.md](../TODO.md). Do not expose the app as an acceptance-complete release or deploy it to existing infrastructure without explicit authorization.
