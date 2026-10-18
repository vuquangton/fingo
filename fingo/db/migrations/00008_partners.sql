-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- The businesses a client trades with.
--
-- Without this the ledger can say TK 131 is 500,000,000 but not who owes it.
-- TT133 keeps Sổ chi tiết TK 131 by customer, so the customer is a dimension of
-- the journal line and not a note beside it.

CREATE TABLE IF NOT EXISTS partners (
    id VARCHAR(36) PRIMARY KEY,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    tax_code VARCHAR(32) NULL,
    address VARCHAR(500) NULL,
    kind ENUM('CUSTOMER', 'SUPPLIER', 'BOTH') NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_partners_code (code),
    -- A tax code identifies one business, so two partners must not share one.
    -- NULL is exempt and repeats: a small customer may have no tax code, and
    -- many of them may.
    UNIQUE KEY uq_partners_tax_code (tax_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- The partner dimension on a journal line. NULL for the many lines that are not
-- owed by or to anyone in particular — cash, inventory, VAT.
ALTER TABLE voucher_lines
    ADD COLUMN partner_id VARCHAR(36) NULL AFTER account_id,
    ADD CONSTRAINT fk_voucher_lines_partner FOREIGN KEY (partner_id) REFERENCES partners(id);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE voucher_lines
    DROP FOREIGN KEY fk_voucher_lines_partner,
    DROP COLUMN partner_id;

DROP TABLE IF EXISTS partners;
