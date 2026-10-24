#!/usr/bin/env bash
# Disposable GitHub-runner smoke only. Does not use a configured real database.
set -euo pipefail
if [[ ${GITHUB_ACTIONS:-} != true ]]; then
  echo "This synthetic Compose fixture is restricted to disposable GitHub CI runners." >&2
  exit 1
fi
work=$(mktemp -d)
export WATCHLEDGER_SYNTHETIC_OIDC=1
export SMOKE_DIR=$work SMOKE_UID=$(id -u) SMOKE_GID=$(id -g)
export DATABASE_URL='postgres://watchledger:synthetic-ci-only@postgres:5432/watchledger_smoke?sslmode=disable'
export PUBLIC_URL=http://127.0.0.1:8080 OIDC_ISSUER=https://oidc:8443 OIDC_CLIENT_ID=watchledger-smoke
export OIDC_CLIENT_SECRET=synthetic-ci-only OIDC_ADMISSION_CLAIM=roles OIDC_ADMISSION_VALUE=member OIDC_ADMIN_CLAIM=roles OIDC_ADMIN_VALUE=admin
export SESSION_KEY=$(head -c 32 /dev/urandom | base64)
export TMDB_TOKEN='' ANIDB_CLIENT='' ANIDB_CLIENT_VERSION=''
project="watchledger-ci-${GITHUB_RUN_ID:?}-${GITHUB_RUN_ATTEMPT:-1}"
compose=(docker compose -p "$project" -f deploy/compose.yaml -f deploy/smoke/compose.yaml)
cleanup() {
  result=$?
  if [[ $result != 0 ]]; then "${compose[@]}" logs --no-color --tail=80 app oidc || true; fi
  if ! "${compose[@]}" down --volumes --remove-orphans; then result=1; fi
  rm -rf "$work"
  exit "$result"
}
trap cleanup EXIT
CGO_ENABLED=0 go build -o "$work/oidc" ./internal/smoketest/oidc
WATCHLEDGER_SYNTHETIC_OIDC=1 "$work/oidc" -generate -dir "$work"
"${compose[@]}" config --quiet
"${compose[@]}" up -d --no-build
ready() {
  for attempt in $(seq 1 60); do
    if curl --silent --fail "$PUBLIC_URL/readyz" >/dev/null; then return; fi
    sleep 2
  done
  echo 'Application did not become ready' >&2
  return 1
}
ready
bash scripts/http-smoke.sh "$work"
"${compose[@]}" restart app
ready
curl --fail --silent -b "$work/cookies" "$PUBLIC_URL/api/library" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert any(x["id"]=="tmdb:movie:900001" and x["base"]=="completed" for x in data)'
curl --fail --silent -b "$work/cookies" "$PUBLIC_URL/api/library/tmdb:movie:900001/history" | python3 -c 'import json,sys; data=json.load(sys.stdin); assert len(data)==1 and data[0]["watched_date"]=="2026-01-02" and data[0]["watched_time"] is None'
# Fixed synthetic identities use verified OIDC claims, never database role edits.
for identity in second admin; do
  curl --fail --silent -D "$work/identity-headers" -c "$work/cookies-$identity" "$PUBLIC_URL/auth/login" -o /dev/null
  fixture_authorize=$(python3 - "$work/identity-headers" "$identity" <<'PYCODE'
import sys,urllib.parse
headers=open(sys.argv[1]).read().splitlines()
url=next(line.split(':',1)[1].strip() for line in headers if line.lower().startswith('location:'))
p=urllib.parse.urlsplit(url)
assert p.scheme=='https' and p.hostname=='localhost' and p.port==8443 and p.path=='/authorize'
assert sys.argv[2] in ('second','admin')
q=urllib.parse.parse_qsl(p.query);q.append(('fixture_user',sys.argv[2]))
print(urllib.parse.urlunsplit((p.scheme,p.netloc,p.path,urllib.parse.urlencode(q),'')))
PYCODE
)
  curl --fail --silent --location --max-redirs 5 --cacert "$work/ca.crt" -c "$work/cookies-$identity" -b "$work/cookies-$identity" "$fixture_authorize" > /dev/null
  unset fixture_authorize
  curl --fail --silent -b "$work/cookies-$identity" "$PUBLIC_URL/api/library" | python3 -c 'import json,sys;assert json.load(sys.stdin)==[]'
done
npm run test:browser
echo 'COMPOSE_LOGIN_TRACKING_RESTART_BROWSER_VERIFIED'
