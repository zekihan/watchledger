CREATE TABLE anidb_title_index (
 aid text PRIMARY KEY CHECK(aid ~ '^[1-9][0-9]*$'),
 titles jsonb NOT NULL CHECK(jsonb_typeof(titles)='array')
);
CREATE TABLE anidb_title_index_state (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 checksum text NOT NULL,
 entry_count integer NOT NULL CHECK(entry_count>0),
 imported_at timestamptz NOT NULL DEFAULT now()
);
