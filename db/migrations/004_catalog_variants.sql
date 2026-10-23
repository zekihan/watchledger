CREATE TABLE catalog_localizations (
 catalog_id text NOT NULL REFERENCES catalog(id), language text NOT NULL,
 metadata jsonb NOT NULL, fetched_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(catalog_id,language)
);
CREATE TABLE season_cache (
 catalog_id text NOT NULL REFERENCES catalog(id), season integer NOT NULL, language text NOT NULL,
 metadata jsonb NOT NULL, fetched_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(catalog_id,season,language)
);
