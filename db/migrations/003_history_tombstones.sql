-- Explicit app unwatch decisions must win over later restore/import data.
CREATE TABLE history_tombstones (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 catalog_id text NOT NULL REFERENCES catalog(id), episode_key text NOT NULL DEFAULT '',
 deleted_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(user_id,catalog_id,episode_key)
);
