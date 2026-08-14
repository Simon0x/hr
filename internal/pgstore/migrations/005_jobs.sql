CREATE TABLE jobs (
    id                BIGSERIAL PRIMARY KEY,
    step_key          TEXT NOT NULL,
    department        TEXT NOT NULL,
    because           TEXT NOT NULL,
    input             TEXT NOT NULL,
    risk              TEXT NOT NULL DEFAULT '',
    reversibility     TEXT NOT NULL DEFAULT '',
    confidence        TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending','claimed','running','done','failed','escalated')),
    claimed_by        TEXT,
    lease_token       UUID,
    claimed_at        TIMESTAMPTZ,
    lease_expires_at  TIMESTAMPTZ,
    attempts          INT NOT NULL DEFAULT 0,
    result_digest     TEXT,
    ledger_seq        BIGINT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX jobs_step_key_active_idx ON jobs (step_key)
    WHERE status IN ('pending','claimed','running','escalated');
CREATE INDEX jobs_claim_idx ON jobs (department, risk DESC, created_at)
    WHERE status = 'pending';
CREATE INDEX jobs_lease_idx ON jobs (lease_expires_at)
    WHERE status IN ('claimed','running');
