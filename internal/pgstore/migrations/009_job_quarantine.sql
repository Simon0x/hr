ALTER TABLE jobs DROP CONSTRAINT jobs_status_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_status_check
    CHECK (status IN ('pending','claimed','running','done','failed','escalated','quarantined'));

DROP INDEX jobs_step_key_active_idx;
CREATE UNIQUE INDEX jobs_step_key_active_idx ON jobs (step_key)
    WHERE status IN ('pending','claimed','running','escalated','quarantined');

ALTER TABLE jobs ADD COLUMN quarantined_reason TEXT;
