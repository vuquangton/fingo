-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Audit columns on vouchers, and a posting date that means what it says.
--
-- Before this, posted_date was NOT NULL while is_posted defaulted to FALSE, so
-- a draft had to carry a posting date and the column could not be trusted. There
-- was also no column recording who posted an entry, which made the ledger
-- unattributable — see docs/adr/0002-posting-lifecycle.md.
--
-- posted_by / created_by are plain identifiers, not foreign keys: there is no
-- users table yet, and FinGo runs with a single operator. They become foreign
-- keys when user management is built.

ALTER TABLE vouchers
    MODIFY COLUMN posted_date DATE NULL,
    ADD COLUMN posted_at DATETIME(6) NULL AFTER posted_date,
    ADD COLUMN posted_by VARCHAR(36) NULL AFTER posted_at,
    ADD COLUMN created_by VARCHAR(36) NULL AFTER posted_by;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE vouchers
    DROP COLUMN created_by,
    DROP COLUMN posted_by,
    DROP COLUMN posted_at,
    MODIFY COLUMN posted_date DATE NOT NULL;
