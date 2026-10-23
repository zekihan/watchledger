-- Official bounded discovery feeds are cached separately from full anime records.
-- Fetching a feed must not create catalog identities or personal library entries.
CREATE TABLE anidb_feeds (
 name text PRIMARY KEY CHECK(name='hotanime'),
 document bytea NOT NULL,
 fetched_at timestamptz NOT NULL DEFAULT now()
);
