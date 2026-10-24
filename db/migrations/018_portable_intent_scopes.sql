-- Preserve watch intent ordering across backup generations without allowing
-- imported watches to override destination-local unwatch decisions.
ALTER TABLE history ADD COLUMN intent_scope text[] NOT NULL DEFAULT '{}';
ALTER TABLE history_tombstones ADD COLUMN intent_scope text[] NOT NULL DEFAULT '{}';
