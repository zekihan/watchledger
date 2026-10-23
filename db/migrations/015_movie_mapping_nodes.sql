ALTER TABLE mappings ADD COLUMN kind text NOT NULL DEFAULT 'episodes' CHECK(kind IN ('episodes','movie-aliases'));
ALTER TABLE mapping_proposals ADD COLUMN kind text NOT NULL DEFAULT 'episodes' CHECK(kind IN ('episodes','movie-aliases'));
ALTER TABLE mapping_revision_history ADD COLUMN kind text NOT NULL DEFAULT 'episodes' CHECK(kind IN ('episodes','movie-aliases'));
DROP INDEX one_dataset_mapping_per_anime;
CREATE UNIQUE INDEX one_dataset_mapping_per_anime_kind ON mappings((provenance->>'anime_id'),kind) WHERE provenance->>'source'='animap';

-- A movie remains a catalog identity with a NULL-episode history event. This
-- view exposes that identity to mapping logic without inventing movie episodes.
CREATE VIEW mapping_nodes AS
 SELECT id,catalog_id,source,source_id,season,number,kind,metadata,air_date,air_time,retired FROM episodes
 UNION ALL
 SELECT id,id,source,source_id,0,1,'movie',jsonb_build_object('name',title),NULL::date,NULL::timestamptz,false FROM catalog WHERE kind='movie';
