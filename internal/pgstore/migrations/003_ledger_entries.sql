CREATE TABLE ledger_entries (
    seq            BIGINT PRIMARY KEY,
    prev           CHAR(64) NOT NULL UNIQUE,
    at             TEXT NOT NULL,
    kind           TEXT NOT NULL,
    actor          TEXT NOT NULL,
    has_artifacts  BOOLEAN NOT NULL DEFAULT false,
    artifacts_in   TEXT[] NOT NULL DEFAULT '{}',
    artifacts_out  TEXT[] NOT NULL DEFAULT '{}',
    goal           TEXT NOT NULL DEFAULT '',
    policy         TEXT NOT NULL DEFAULT '',
    outcome        TEXT NOT NULL DEFAULT '',
    detail         TEXT NOT NULL DEFAULT '',
    has_cost       BOOLEAN NOT NULL DEFAULT false,
    cost_tokens    DOUBLE PRECISION,
    cost_seconds   DOUBLE PRECISION,
    cost_model     TEXT NOT NULL DEFAULT ''
);
