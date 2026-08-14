CREATE TABLE identities (
    spiffe_id      TEXT PRIMARY KEY,
    display_name   TEXT NOT NULL,
    token_hash     TEXT NOT NULL,
    departments    TEXT[] NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
