CREATE TABLE source_catalog_state (
  source_id text PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('syncing','ready','error')),
  active_generation bigint NOT NULL DEFAULT 0,
  updated_at timestamptz,
  sync_started_at timestamptz,
  error_code text,
  counts jsonb NOT NULL DEFAULT '{"live":0,"movie":0,"series":0}'::jsonb
);

CREATE TABLE source_catalog_categories (
  source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  generation bigint NOT NULL,
  kind text NOT NULL CHECK (kind IN ('live','movie','series')),
  id text NOT NULL,
  name text NOT NULL,
  PRIMARY KEY (source_id,generation,kind,id)
);

CREATE TABLE source_catalog_entries (
  source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  generation bigint NOT NULL,
  kind text NOT NULL CHECK (kind IN ('live','movie','series')),
  id text NOT NULL,
  title text NOT NULL,
  title_search text NOT NULL,
  category_id text NOT NULL DEFAULT '',
  language text NOT NULL DEFAULT 'und',
  extension text NOT NULL DEFAULT '',
  content_key text NOT NULL,
  image_ciphertext bytea,
  PRIMARY KEY (source_id,generation,kind,id)
);
CREATE INDEX source_catalog_entries_search_idx ON source_catalog_entries(source_id,generation,kind,title_search text_pattern_ops);
CREATE INDEX source_catalog_entries_filters_idx ON source_catalog_entries(source_id,generation,kind,category_id,language,title_search);

CREATE TABLE source_catalog_languages (
  source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  generation bigint NOT NULL,
  kind text NOT NULL CHECK (kind IN ('live','movie','series')),
  id text NOT NULL,
  name text NOT NULL,
  PRIMARY KEY (source_id,generation,kind,id)
);
