CREATE TABLE anidb_documents (
 aid text PRIMARY KEY CHECK(aid ~ '^[1-9][0-9]*$'),
 document bytea NOT NULL,
 fetched_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE provider_request_limits (
 key text PRIMARY KEY,
 next_allowed_at timestamptz NOT NULL
);
