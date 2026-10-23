-- User-confirmed source episode identities survive unwatch decisions and reimports.
ALTER TABLE episodes ADD CONSTRAINT episodes_id_catalog_unique UNIQUE(id,catalog_id);
CREATE TABLE simkl_episode_links (
 user_id uuid NOT NULL,
 source_key text NOT NULL CHECK(length(source_key) BETWEEN 1 AND 256),
 catalog_id text NOT NULL,
 episode_id text NOT NULL,
 PRIMARY KEY(user_id,source_key),
 FOREIGN KEY(user_id,catalog_id) REFERENCES library(user_id,catalog_id) ON DELETE CASCADE,
 FOREIGN KEY(episode_id,catalog_id) REFERENCES episodes(id,catalog_id)
);
