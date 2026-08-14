CREATE TABLE workers (
    identity      TEXT PRIMARY KEY,
    departments   TEXT[] NOT NULL DEFAULT '{}',
    last_seen_at  TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'offline'
);
