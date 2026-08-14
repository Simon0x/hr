ALTER TABLE ledger_entries ADD COLUMN cost_currency TEXT NOT NULL DEFAULT '';
ALTER TABLE ledger_entries ADD COLUMN cost_amount DOUBLE PRECISION;
