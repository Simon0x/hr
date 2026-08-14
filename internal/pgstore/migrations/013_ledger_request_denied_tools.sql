ALTER TABLE ledger_entries ADD COLUMN request_denied_tools TEXT[] NOT NULL DEFAULT '{}';
