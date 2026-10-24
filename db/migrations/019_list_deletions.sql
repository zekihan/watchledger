-- Preserve explicit ordinary-list deletions across additive portable restores.
CREATE TABLE list_tombstones (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 list_id uuid NOT NULL,
 deleted_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,list_id)
);
-- Older settings accepted a Go-only pseudo-zone, unusable by the UI and SQL.
UPDATE users SET settings=jsonb_set(settings,'{timezone}','"UTC"'::jsonb)
WHERE settings->>'timezone' IN ('Local','') OR settings->>'timezone' LIKE 'posix/%' OR settings->>'timezone' LIKE 'right/%';
