-- Empty owner means shared/visible to everyone (system-derived work from the
-- Repopulate loop); a non-empty owner is a real identity's spiffe_id and the
-- job is private to them (manually run from the web UI). Exceptions reuse
-- the existing artifacts.created_by column for the same distinction instead
-- of a new column - see internal/pgstore/exceptions.go.
ALTER TABLE jobs ADD COLUMN owner TEXT NOT NULL DEFAULT '';
