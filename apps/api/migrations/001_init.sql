CREATE TABLE users (
  id text PRIMARY KEY,
  username text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role = 'admin'),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  token_hash bytea PRIMARY KEY,
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  idle_expires_at timestamptz NOT NULL,
  absolute_expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);

CREATE TABLE favorites (
  id text PRIMARY KEY,
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content_key text NOT NULL,
  title text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('live','movie','episode')),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, content_key)
);

CREATE TABLE progress (
  user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content_key text NOT NULL,
  title text NOT NULL,
  kind text NOT NULL CHECK (kind IN ('live','movie','episode')),
  position_seconds double precision NOT NULL CHECK (position_seconds >= 0),
  duration_seconds double precision NOT NULL CHECK (duration_seconds >= 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, content_key)
);
