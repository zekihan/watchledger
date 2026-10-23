CREATE TABLE list_source_tombstones (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 source text NOT NULL CHECK(source='simkl-custom-list'),
 source_id text NOT NULL CHECK(source_id ~ '^[1-9][0-9]*$'),
 deleted_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,source,source_id)
);
