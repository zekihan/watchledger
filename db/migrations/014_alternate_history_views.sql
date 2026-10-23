-- Private history containers for alternate presentations, without fabricated
-- library statuses or ratings. Mapping consent is deliberately not stored here.
CREATE TABLE alternate_history_views (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 catalog_id text NOT NULL REFERENCES catalog(id),
 anchor_id text NOT NULL,
 PRIMARY KEY(user_id,catalog_id),
 FOREIGN KEY(user_id,anchor_id) REFERENCES library(user_id,catalog_id),
 CHECK(catalog_id<>anchor_id)
);
