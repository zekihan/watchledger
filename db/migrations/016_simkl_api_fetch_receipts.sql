-- Only fetched list data is archived. Access tokens and client secrets are not
-- persisted, exported, logged, refreshed, or used for ongoing synchronization.
ALTER TABLE imports ADD COLUMN api_fetched_at timestamptz;
ALTER TABLE imports ADD COLUMN api_list_type text;
CREATE INDEX imports_api_list_cache ON imports(user_id,api_fetched_at) WHERE api_fetched_at IS NOT NULL;
