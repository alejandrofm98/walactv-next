ALTER TABLE source_catalog_entries
  ADD COLUMN release_year integer CHECK (release_year BETWEEN 1888 AND 2200),
  ADD COLUMN tmdb_id bigint CHECK (tmdb_id > 0),
  ADD COLUMN imdb_id text CHECK (imdb_id ~ '^tt[0-9]{7,10}$');

CREATE TABLE tmdb_metadata_cache (
  kind text NOT NULL CHECK (kind IN ('movie','series')),
  canonical_id bigint NOT NULL CHECK (canonical_id > 0),
  language text NOT NULL CHECK (language = 'es-ES'),
  payload jsonb NOT NULL,
  fetched_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (kind, canonical_id, language)
);

CREATE TABLE tmdb_season_cache (
  kind text NOT NULL CHECK (kind = 'series'),
  canonical_id bigint NOT NULL CHECK (canonical_id > 0),
  language text NOT NULL CHECK (language = 'es-ES'),
  season_number integer NOT NULL CHECK (season_number >= 0),
  payload jsonb NOT NULL,
  fetched_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (kind, canonical_id, language, season_number),
  FOREIGN KEY (kind, canonical_id, language)
    REFERENCES tmdb_metadata_cache(kind, canonical_id, language) ON DELETE CASCADE
);

CREATE TABLE tmdb_item_resolutions (
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('movie','series')),
  item_id text NOT NULL,
  fingerprint text NOT NULL,
  status text NOT NULL CHECK (status IN ('resolved','unresolved','ambiguous')),
  canonical_id bigint,
  checked_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, source_id, kind, item_id),
  CHECK ((status = 'resolved' AND canonical_id IS NOT NULL) OR (status <> 'resolved' AND canonical_id IS NULL))
);

CREATE TABLE tmdb_provider_cooldowns (
  scope text PRIMARY KEY,
  retry_after timestamptz NOT NULL
);
