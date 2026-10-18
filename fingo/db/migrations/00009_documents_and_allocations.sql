-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- The business documents a voucher records, and the payments that settle them.
--
-- A voucher says money moved. A document says what the money is for. Without it
-- the ledger totals TK 131 but cannot age it, and "how much is overdue?" has no
-- answer — which is the question a trading SME actually asks.

CREATE TABLE IF NOT EXISTS documents (
    id VARCHAR(36) PRIMARY KEY,
    partner_id VARCHAR(36) NOT NULL,
    voucher_id VARCHAR(36) NOT NULL,
    kind ENUM('SALES_INVOICE', 'PURCHASE_INVOICE', 'CREDIT_NOTE') NOT NULL,
    reference VARCHAR(64) NOT NULL,
    document_date DATE NOT NULL,
    due_date DATE NULL,
    amount DECIMAL(19, 0) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- One paper document per partner. The same number from two suppliers is two
    -- documents; the same number twice from one partner is a double entry.
    UNIQUE KEY uq_documents_reference (partner_id, kind, reference),
    -- Aging reads a partner's documents oldest first, which is the order the
    -- index stores them in.
    KEY ix_documents_partner_date (partner_id, document_date),
    CONSTRAINT fk_documents_partner FOREIGN KEY (partner_id) REFERENCES partners(id),
    CONSTRAINT fk_documents_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS allocations (
    id VARCHAR(36) PRIMARY KEY,
    document_id VARCHAR(36) NOT NULL,
    voucher_id VARCHAR(36) NOT NULL,
    amount DECIMAL(19, 0) NOT NULL,
    allocated_at DATETIME(6) NOT NULL,
    -- A payment applies to a given document once. Applying it twice is a
    -- double-count that no report could detect after the fact.
    UNIQUE KEY uq_allocations (document_id, voucher_id),
    KEY ix_allocations_voucher (voucher_id),
    CONSTRAINT fk_allocations_document FOREIGN KEY (document_id) REFERENCES documents(id),
    CONSTRAINT fk_allocations_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS allocations;
DROP TABLE IF EXISTS documents;
