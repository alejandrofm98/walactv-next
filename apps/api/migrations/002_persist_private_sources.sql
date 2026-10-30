CREATE TABLE sources (
  id text PRIMARY KEY,
  user_id text NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  label text NOT NULL,
  account_key text NOT NULL,
  fingerprint text NOT NULL,
  max_connections integer NOT NULL CHECK (max_connections BETWEEN 1 AND 64),
  secret_ciphertext bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sources_user_id_idx ON sources(user_id);

CREATE TABLE source_catalogs (
  source_id text NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  catalog_key text NOT NULL,
  payload jsonb NOT NULL,
  expires_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (source_id, catalog_key)
);
