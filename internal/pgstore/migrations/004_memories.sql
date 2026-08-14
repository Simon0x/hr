CREATE TABLE memories (
    digest         TEXT PRIMARY KEY,
    claim          TEXT NOT NULL,
    confidence     TEXT NOT NULL,
    source         TEXT NOT NULL,
    owner          TEXT NOT NULL,
    learned_at     TEXT NOT NULL,
    true_from      TEXT NOT NULL DEFAULT '',
    true_until     TEXT NOT NULL DEFAULT '',
    last_verified  TEXT NOT NULL,
    verify_every   TEXT NOT NULL DEFAULT '',
    scope          TEXT[] NOT NULL DEFAULT '{}',
    data_class     TEXT NOT NULL DEFAULT '',
    supersedes     TEXT NOT NULL DEFAULT '',
    quarantined    BOOLEAN NOT NULL DEFAULT false,
    promoted_to    TEXT NOT NULL DEFAULT '',
    embedding      vector(1024),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX memories_scope_idx ON memories USING GIN (scope);
