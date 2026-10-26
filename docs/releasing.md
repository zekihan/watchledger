# Releasing Watchledger

Releases follow the Docker Hub/GHCR and OCI Helm conventions used by Accessrelay and Mailvault. Release tags are explicit; ordinary branch pushes and pull requests only validate the application.

## Version and validation

Keep `VERSION`, both npm package versions and lockfile metadata, `Chart.yaml` version/appVersion, chart image tag and Compose image tag aligned. Use a stable semantic version and a matching `v`-prefixed Git tag. Add release notes in `docs/releases/<version>.md`.

Run:

```sh
npm ci
./scripts/release-version.sh
make lint test build
helm lint deploy/helm/watchledger --set existingSecret=fixture --set publicURL=https://watch.example --set oidc.issuer=https://identity.example
```

The Release workflow reruns the complete CI matrix at the tagged commit, including database/race tests, container startup, OIDC, persistence and synthetic browser acceptance. No artifacts publish if those checks fail. Unresolved real-provider/device acceptance must remain explicit in release notes and the feature tracker.

## Credentials

`DOCKER_USERNAME` and `DOCKER_PASSWORD` are repository Actions secrets provisioned from the existing Docker Hub credentials through the Terraform `git` root's `watchledger_release.tf`. Never set or commit credential values here. GHCR and GitHub releases use the built-in `GITHUB_TOKEN` with job-scoped package/content write permissions, requiring no additional stored token.

## Publish and verify

After the validated release commit is published, explicitly publish its matching tag to start `.github/workflows/release.yml`. Git pushes require the repository owner's explicit instruction.

The workflow publishes Linux amd64/arm64 images to `docker.io/zekihan/watchledger` and `ghcr.io/zekihan/watchledger`, then packages and publishes the chart to `oci://ghcr.io/zekihan/charts`. It checks image platforms and pulls/renders the chart using empty credential stores before creating the GitHub release with the verified chart archive and checksum. If a first GHCR publication defaults to private visibility, set the image/chart package visibility to public and rerun the failed verification job before declaring the release usable.

After publication, repeat the anonymous checks locally with:

```sh
./scripts/verify-release.sh "$(cat VERSION)"
```

No application credentials, private imports, database state, or infrastructure deployments belong in this workflow. See the [deployment guide](deployment.md) for installation and backup/recovery.
