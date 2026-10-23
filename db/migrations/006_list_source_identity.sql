ALTER TABLE lists ADD COLUMN source text;
ALTER TABLE lists ADD COLUMN source_id text;
ALTER TABLE lists ADD CONSTRAINT lists_source_pair CHECK (
 (source IS NULL AND source_id IS NULL) OR
 (source IS NOT NULL AND source_id IS NOT NULL AND source='simkl-custom-list' AND source_id ~ '^[1-9][0-9]*$')
);
CREATE UNIQUE INDEX lists_source_identity ON lists(user_id,source,source_id) WHERE source IS NOT NULL;
