CREATE TABLE users (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), issuer text NOT NULL, subject text NOT NULL,
 name text NOT NULL DEFAULT '', admin boolean NOT NULL DEFAULT false,
 settings jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(issuer,subject)
);
CREATE TABLE sessions (
 token_hash text PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token jsonb NOT NULL, expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE catalog (
 id text PRIMARY KEY, source text NOT NULL, source_id text NOT NULL, kind text NOT NULL CHECK(kind IN ('movie','tv','anime','person')),
 title text NOT NULL, metadata jsonb NOT NULL DEFAULT '{}', fetched_at timestamptz NOT NULL DEFAULT now(), UNIQUE(source,source_id,kind)
);
CREATE TABLE episodes (
 id text PRIMARY KEY, catalog_id text NOT NULL REFERENCES catalog(id), source text NOT NULL, source_id text NOT NULL,
 season integer NOT NULL DEFAULT 0, number integer NOT NULL, kind text NOT NULL DEFAULT 'regular',
 metadata jsonb NOT NULL DEFAULT '{}', air_date date, air_time timestamptz, retired boolean NOT NULL DEFAULT false,
 CHECK(air_date IS NULL OR air_time IS NULL), UNIQUE(source,source_id)
);
CREATE TABLE statuses (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100), base text NOT NULL CHECK(base IN ('planned','watching','completed','hold','dropped')),
 used boolean NOT NULL DEFAULT false, UNIQUE(user_id,id), UNIQUE(user_id,name)
);
CREATE TABLE library (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, catalog_id text NOT NULL REFERENCES catalog(id),
 base text NOT NULL DEFAULT 'planned' CHECK(base IN ('planned','watching','completed','hold','dropped')),
 status_id uuid, rating integer CHECK(rating BETWEEN 1 AND 10), position integer NOT NULL DEFAULT 0,
 anime_view text CHECK(anime_view IN ('native','show')), added_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,catalog_id), FOREIGN KEY(user_id,status_id) REFERENCES statuses(user_id,id)
);
CREATE TABLE history (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 catalog_id text NOT NULL REFERENCES catalog(id), episode_id text REFERENCES episodes(id),
 watched_date date, watched_time timestamptz, provenance jsonb NOT NULL DEFAULT '{}',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(watched_date IS NULL OR watched_time IS NULL), UNIQUE NULLS NOT DISTINCT(user_id,catalog_id,episode_id)
);
CREATE TABLE lists (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100), kind text NOT NULL CHECK(kind IN ('manual','smart')),
 rule jsonb, sort text NOT NULL DEFAULT 'title', updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(user_id,id)
);
CREATE TABLE list_members (
 user_id uuid NOT NULL, list_id uuid NOT NULL, catalog_id text NOT NULL, position integer NOT NULL DEFAULT 0,
 PRIMARY KEY(user_id,list_id,catalog_id), FOREIGN KEY(user_id,list_id) REFERENCES lists(user_id,id) ON DELETE CASCADE,
 FOREIGN KEY(user_id,catalog_id) REFERENCES library(user_id,catalog_id)
);
CREATE TABLE list_dependencies (
 user_id uuid NOT NULL, list_id uuid NOT NULL, depends_on uuid NOT NULL,
 PRIMARY KEY(user_id,list_id,depends_on), CHECK(list_id<>depends_on),
 FOREIGN KEY(user_id,list_id) REFERENCES lists(user_id,id) ON DELETE CASCADE,
 FOREIGN KEY(user_id,depends_on) REFERENCES lists(user_id,id)
);
CREATE TABLE mappings (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), revision text NOT NULL, provenance jsonb NOT NULL,
 relationships jsonb NOT NULL, requires_consent boolean NOT NULL, reviewed boolean NOT NULL DEFAULT false,
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE mapping_consents (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, mapping_id uuid NOT NULL REFERENCES mappings(id),
 revision text NOT NULL, confirmed_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(user_id,mapping_id)
);
CREATE TABLE imports (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind text NOT NULL CHECK(kind IN ('simkl','restore')), checksum text NOT NULL, archive jsonb NOT NULL,
 preview jsonb NOT NULL DEFAULT '{}', state text NOT NULL DEFAULT 'preview', created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(user_id,checksum), UNIQUE(user_id,id)
);
CREATE TABLE import_reviews (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL, import_id uuid NOT NULL,
 item jsonb NOT NULL, reason text NOT NULL, resolved boolean NOT NULL DEFAULT false,
 FOREIGN KEY(user_id,import_id) REFERENCES imports(user_id,id) ON DELETE CASCADE
);
CREATE TABLE notices (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind text NOT NULL, data jsonb NOT NULL DEFAULT '{}', read_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE jobs (
 id text PRIMARY KEY, kind text NOT NULL, payload jsonb NOT NULL, run_at timestamptz NOT NULL DEFAULT now(),
 attempts integer NOT NULL DEFAULT 0, last_error text, locked_until timestamptz
);
CREATE TABLE availability (
 catalog_id text NOT NULL REFERENCES catalog(id), country text NOT NULL, offers jsonb NOT NULL,
 fetched_at timestamptz NOT NULL DEFAULT now(), used_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(catalog_id,country)
);
