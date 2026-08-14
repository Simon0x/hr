ALTER TABLE ledger_entries ADD COLUMN has_request BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ledger_entries ADD COLUMN request_harness TEXT NOT NULL DEFAULT '';
ALTER TABLE ledger_entries ADD COLUMN request_prompt_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE ledger_entries ADD COLUMN request_tools TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE ledger_entries ADD COLUMN request_procedure TEXT NOT NULL DEFAULT '';
ALTER TABLE ledger_entries ADD COLUMN request_procedure_digest TEXT NOT NULL DEFAULT '';
