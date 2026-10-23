-- name: UpsertUser :one
INSERT INTO users (issuer,subject,name,admin) VALUES ($1,$2,$3,$4)
ON CONFLICT (issuer,subject) DO UPDATE SET name=EXCLUDED.name,admin=EXCLUDED.admin
RETURNING id,issuer,subject,name,admin,settings,created_at;

-- name: UserBySession :one
SELECT u.id,u.issuer,u.subject,u.name,u.admin,u.settings,u.created_at,s.token,s.expires_at
FROM users u JOIN sessions s ON s.user_id=u.id WHERE s.token_hash=$1 AND s.created_at > now() - interval '12 hours';

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash=$1;

-- name: SaveSession :exec
INSERT INTO sessions(token_hash,user_id,token,expires_at) VALUES($1,$2,$3,$4);

-- name: RefreshSession :exec
UPDATE sessions SET token=$2,expires_at=$3 WHERE token_hash=$1;

-- name: SetAdmin :exec
UPDATE users SET admin=$2 WHERE id=$1;
