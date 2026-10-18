-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- The client registry, and the client column on everything a client owns.
--
-- This migration deliberately reverses one clause of a decision that is already
-- on record. ADR 0001 says, under "Not decided here": one database per client,
-- "so there is no company_id column", and requirements-v1.md §1.1 argues the
-- same. That was right about the *outer* boundary and wrong to conclude there
-- is only one. The connection still decides which database is reachable; the
-- column decides which client a row inside it belongs to, and it is what lets a
-- repository refuse a query that named nobody. See ADR 0003.
--
-- The registry itself holds no financial data. It is the practice's list: who
-- the clients are, what they are called, and whether they are still served.
-- internal/client is the model; this is where it is stored.

CREATE TABLE IF NOT EXISTS clients (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    -- Mã số thuế. NULL, not '', for a client engaged before its paperwork is
    -- to hand: safe rather than legal, so a unique key can exempt it and let
    -- several such clients coexist. A Go adapter must write NULL for an empty
    -- TaxID, or the second client with no tax code collides on the key below.
    tax_id VARCHAR(32) NULL,
    -- DATETIME(6) in UTC, like every other instant in this schema. A client is
    -- registered at a moment, not on a day. See accounting-review.md.
    created_at DATETIME(6) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    -- One company, one registration. Registering the same mã số thuế twice
    -- would split one company's books in two, which is a mistake nobody can
    -- undo afterwards by looking at the rows.
    UNIQUE KEY uq_clients_tax_id (tax_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- The client column, on every table that holds a client's data.
--
-- NULLABLE on purpose. A row written before this migration belongs to the one
-- client whose database it is in, and nothing in the row says which — so it
-- carries NULL until something fills it in. A scoped repository adopts a row
-- with no owner and refuses a row owned by somebody else; NULL is therefore the
-- honest encoding of "not yet stated", where a NOT NULL DEFAULT '' would file
-- every legacy row under a client called nothing and hide it from everybody.
--
-- New rows must state their client. That is enforced at the repository layer,
-- where a query without a Scope is refused before it runs, and not by making
-- the column NOT NULL: see ADR 0003 for why the constraint and the guard are
-- not the same thing.
--
-- The foreign key to clients(id) creates the index that reads by client; it
-- also makes a client_id that names no registered client impossible to write,
-- which is the difference between a column and a scoped column.

ALTER TABLE accounts
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_accounts_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE accounting_periods
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_accounting_periods_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE vouchers
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_vouchers_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE voucher_lines
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_voucher_lines_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE voucher_sequences
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_voucher_sequences_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE idempotency_keys
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_idempotency_keys_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE partners
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_partners_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE documents
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_documents_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE allocations
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_allocations_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE stock_movements
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_stock_movements_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE inventory_policies
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_inventory_policies_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE accounting_period_locks
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_accounting_period_locks_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE voucher_series
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_voucher_series_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE trade_partners
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_trade_partners_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE arap_documents
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_arap_documents_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE arap_lines
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_arap_lines_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE arap_allocations
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_arap_allocations_client FOREIGN KEY (client_id) REFERENCES clients(id);

ALTER TABLE arap_credits
    ADD COLUMN client_id VARCHAR(36) NULL,
    ADD CONSTRAINT fk_arap_credits_client FOREIGN KEY (client_id) REFERENCES clients(id);

-- NOT done here, and named so the gap is a task rather than a surprise: the
-- natural keys below are still global, so two clients sharing one schema could
-- not both use account 111, voucher book "NK", or partner code "KH001". In a
-- database holding one client they are the same thing as a per-client key, so
-- nothing is wrong today. The moment the practice database is split out — or
-- two clients share a schema for any reason — each becomes composite with
-- client_id as its leading column:
--
--   accounts.code, uq_vouchers_number, voucher_sequences (book, fiscal_year),
--   idempotency_keys (key_value), uq_partners_code, uq_partners_tax_code,
--   uq_voucher_series, inventory_policies (fiscal_year), accounting_periods
--   (year, month)
--
-- Restructuring them now would be a second kind of change in the same slice,
-- and one this migration does not need.

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

-- The foreign keys go first: they are what points at clients(id), and the table
-- cannot be dropped while anything references it.

ALTER TABLE arap_credits
    DROP FOREIGN KEY fk_arap_credits_client,
    DROP COLUMN client_id;

ALTER TABLE arap_allocations
    DROP FOREIGN KEY fk_arap_allocations_client,
    DROP COLUMN client_id;

ALTER TABLE arap_lines
    DROP FOREIGN KEY fk_arap_lines_client,
    DROP COLUMN client_id;

ALTER TABLE arap_documents
    DROP FOREIGN KEY fk_arap_documents_client,
    DROP COLUMN client_id;

ALTER TABLE trade_partners
    DROP FOREIGN KEY fk_trade_partners_client,
    DROP COLUMN client_id;

ALTER TABLE voucher_series
    DROP FOREIGN KEY fk_voucher_series_client,
    DROP COLUMN client_id;

ALTER TABLE accounting_period_locks
    DROP FOREIGN KEY fk_accounting_period_locks_client,
    DROP COLUMN client_id;

ALTER TABLE inventory_policies
    DROP FOREIGN KEY fk_inventory_policies_client,
    DROP COLUMN client_id;

ALTER TABLE stock_movements
    DROP FOREIGN KEY fk_stock_movements_client,
    DROP COLUMN client_id;

ALTER TABLE allocations
    DROP FOREIGN KEY fk_allocations_client,
    DROP COLUMN client_id;

ALTER TABLE documents
    DROP FOREIGN KEY fk_documents_client,
    DROP COLUMN client_id;

ALTER TABLE partners
    DROP FOREIGN KEY fk_partners_client,
    DROP COLUMN client_id;

ALTER TABLE idempotency_keys
    DROP FOREIGN KEY fk_idempotency_keys_client,
    DROP COLUMN client_id;

ALTER TABLE voucher_sequences
    DROP FOREIGN KEY fk_voucher_sequences_client,
    DROP COLUMN client_id;

ALTER TABLE voucher_lines
    DROP FOREIGN KEY fk_voucher_lines_client,
    DROP COLUMN client_id;

ALTER TABLE vouchers
    DROP FOREIGN KEY fk_vouchers_client,
    DROP COLUMN client_id;

ALTER TABLE accounting_periods
    DROP FOREIGN KEY fk_accounting_periods_client,
    DROP COLUMN client_id;

ALTER TABLE accounts
    DROP FOREIGN KEY fk_accounts_client,
    DROP COLUMN client_id;

DROP TABLE IF EXISTS clients;
