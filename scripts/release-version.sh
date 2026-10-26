#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
version=$(cat VERSION)
if [[ ! $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo 'VERSION must contain a stable semantic version' >&2
  exit 1
fi
[[ $(sed -n 's/^version: //p' deploy/helm/watchledger/Chart.yaml) == "$version" ]]
[[ $(sed -n 's/^appVersion: "\(.*\)"/\1/p' deploy/helm/watchledger/Chart.yaml) == "$version" ]]
[[ $(sed -n 's/^  tag: "\(.*\)"/\1/p' deploy/helm/watchledger/values.yaml) == "$version" ]]
[[ $(sed -n 's/^    image: ghcr.io\/zekihan\/watchledger://p' deploy/compose.yaml) == "$version" ]]
node --input-type=module - "$version" <<'JS'
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const version = process.argv[2];
for (const file of ['package.json', 'web/package.json', 'package-lock.json']) {
  const pkg = JSON.parse(readFileSync(file, 'utf8'));
  assert.equal(pkg.version, version, file);
  if (pkg.packages) {
    for (const name of ['', 'web']) assert.equal(pkg.packages[name].version, version, `${file}:${name}`);
  }
}
JS
if [[ ${GITHUB_REF:-} == refs/tags/* ]]; then
  [[ $GITHUB_REF == "refs/tags/v$version" ]]
fi
printf '%s\n' "$version"
