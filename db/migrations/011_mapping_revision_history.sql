CREATE TABLE mapping_revision_history (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 mapping_id uuid NOT NULL REFERENCES mappings(id),
 revision text NOT NULL,
 provenance jsonb NOT NULL,
 relationships jsonb NOT NULL,
 recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mapping_revision_history_lookup ON mapping_revision_history(mapping_id,recorded_at);
INSERT INTO mapping_revision_history(mapping_id,revision,provenance,relationships,recorded_at)
 SELECT id,revision,provenance,relationships,updated_at FROM mappings;
CREATE UNIQUE INDEX one_dataset_mapping_per_anime ON mappings((provenance->>'anime_id'))
 WHERE provenance->>'source'='animap';
