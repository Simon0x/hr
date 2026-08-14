CREATE TABLE artifacts (
    digest         TEXT PRIMARY KEY,
    kind           TEXT NOT NULL,
    predicate_type TEXT NOT NULL,
    subject        JSONB NOT NULL,
    predicate      JSONB NOT NULL,
    canonical      BYTEA NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by     TEXT NOT NULL
);

CREATE INDEX artifacts_kind_idx ON artifacts (kind);
