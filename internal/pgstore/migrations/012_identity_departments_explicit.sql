-- An empty departments array now means no departments, not all of them.
-- Identities provisioned before that rule ran unscoped in fact, so they are
-- carried over as explicitly unscoped rather than silently losing access.
UPDATE identities SET departments = '{"*"}' WHERE departments = '{}';

ALTER TABLE identities ADD CONSTRAINT identities_departments_not_empty
    CHECK (cardinality(departments) > 0);
