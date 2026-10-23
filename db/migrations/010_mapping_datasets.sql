CREATE TABLE mapping_datasets (
 checksum text PRIMARY KEY CHECK(checksum ~ '^[a-f0-9]{64}$'),
 version text NOT NULL,
 source_revision text NOT NULL,
 generated_at timestamptz NOT NULL,
 attribution jsonb NOT NULL,
 document bytea NOT NULL,
 imported_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE mapping_dataset_anime (
 dataset_checksum text NOT NULL REFERENCES mapping_datasets(checksum),
 anidb_id bigint NOT NULL CHECK(anidb_id>0),
 data jsonb NOT NULL,
 PRIMARY KEY(dataset_checksum,anidb_id)
);
CREATE TABLE mapping_proposals (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 dataset_checksum text NOT NULL REFERENCES mapping_datasets(checksum),
 anidb_id bigint NOT NULL,
 relationships jsonb NOT NULL,
 unresolved jsonb NOT NULL,
 revision text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE external_episode_links (
 source text NOT NULL CHECK(source='tvdb'),
 source_id bigint NOT NULL CHECK(source_id>0),
 tmdb_episode_id bigint NOT NULL CHECK(tmdb_episode_id>0),
 tmdb_show_id bigint NOT NULL CHECK(tmdb_show_id>0),
 season integer NOT NULL CHECK(season>=0),
 number integer NOT NULL CHECK(number>0),
 checked_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(source,source_id)
);
