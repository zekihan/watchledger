#!/usr/bin/env bash
set -euo pipefail
[[ ${WATCHLEDGER_SYNTHETIC_OIDC:-} == 1 ]] || { echo "Synthetic fixture opt-in required" >&2; exit 1; }
work=${1:?Synthetic fixture directory required}
origin=http://127.0.0.1:8080
[[ $(curl --silent -o /dev/null -w '%{http_code}' "$origin/api/me") == 401 ]]
curl --fail --silent "$origin/manifest.webmanifest" | python3 -c 'import json,sys; m=json.load(sys.stdin); assert m["display"]=="standalone" and len(m["icons"])>=2'
curl --fail --silent --location --max-redirs 5 --cacert "$work/ca.crt" -c "$work/cookies" -b "$work/cookies" "$origin/auth/login" > "$work/login.html"
curl --fail --silent -b "$work/cookies" "$origin/api/me" | python3 -c 'import json,sys; u=json.load(sys.stdin); assert u["Name"]=="Synthetic CI User" and u["Admin"] is False'
[[ $(curl --silent -b "$work/cookies" -o /dev/null -w '%{http_code}' "$origin/api/admin/accounts") == 403 ]]
# A downloaded empty export satisfies the same prerequisite as the user workflow.
curl --fail --silent -b "$work/cookies" "$origin/api/export" > "$work/before.json"
python3 - "$work" <<'PYCODE'
import json,sys,pathlib
p=pathlib.Path(sys.argv[1]); data=json.loads((p/'before.json').read_text())
assert data['version']==1 and data['library']==[]
assert 'sessions' not in data and 'roles' not in data and 'admin' not in data
stamp='2026-01-01T00:00:00Z'
data['catalog']=[dict(id='tmdb:movie:900001',source='tmdb',source_id='900001',kind='movie',title='Synthetic smoke movie',metadata={'en':{'id':900001,'title':'Synthetic smoke movie','runtime':90}},fetched_at=stamp)]
data['library']=[dict(catalog_id='tmdb:movie:900001',base='planned',rating=8,position=0,status_id=None,anime_view=None,added_at=stamp,updated_at=stamp)]
(p/'restore.json').write_text(json.dumps(data))
PYCODE
curl --fail --silent -b "$work/cookies" -H "Origin: $origin" -H 'Content-Type: application/json' --data-binary "@$work/restore.json" "$origin/api/restore/preview" > "$work/preview.json"
preview=$(python3 -c 'import json,sys; p=json.load(open(sys.argv[1])); assert p["additions"]==1 and p["backup_required"];print(p["id"])' "$work/preview.json")
# Preview alone must not import the title.
curl --fail --silent -b "$work/cookies" "$origin/api/library" | python3 -c 'import json,sys;assert json.load(sys.stdin)==[]'
curl --fail --silent -b "$work/cookies" -H "Origin: $origin" -H 'Content-Type: application/json' -d '{}' "$origin/api/restore/$preview/apply" > "$work/apply.json"
curl --fail --silent -b "$work/cookies" -H "Origin: $origin" -H 'Content-Type: application/json' -d '{"watched":true,"date":{"day":"2026-01-02"}}' "$origin/api/library/tmdb:movie:900001/track" > "$work/track.json"
curl --fail --silent -b "$work/cookies" "$origin/api/export" > "$work/after.json"
python3 - "$work/after.json" <<'PYCODE'
import json,sys
s=json.load(open(sys.argv[1])); assert len(s['library'])==1 and s['library'][0]['base']=='completed' and s['library'][0]['rating']==8
assert len(s['history'])==1 and s['history'][0]['episode_id'] is None and s['history'][0]['watched_date']=='2026-01-02' and s['history'][0]['watched_time'] is None
assert 'local_action_at' not in s['history'][0] and 'roles' not in s and 'sessions' not in s
PYCODE
# Cached tracking above succeeded despite no configured external metadata token.
[[ $(curl --silent -b "$work/cookies" -o /dev/null -w '%{http_code}' "$origin/api/catalog/movie/900002") == 502 ]]
echo 'HTTP_LOGIN_PREVIEW_TRACKING_EXPORT_VERIFIED'
