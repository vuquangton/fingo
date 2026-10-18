-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Partners as a directory: a name, which side of the books the business appears
-- on, its tax id, and the terms it trades on.
--
-- NOTE: this coexists with `partners` (00008), which carries a customer code, an
-- address and a kind, and which `voucher_lines.partner_id` points at. They are
-- two models of the same idea and one of them has to go — a directory does not
-- need a NOT NULL UNIQUE code, and a ledger line needs a partner it can point
-- at. That consolidation is a separate decision and is not made here. The name
-- is deliberately distinct so nothing in this migration can clobber the existing
-- table.
--
-- Two differences from `partners` are deliberate and load-bearing:
--
--   * tax_id is indexed but NOT UNIQUE. Rows may share one: a branch filing
--     under its head office, or one legal entity kept under two trading names.
--     A duplicate is reported to the operator as a warning and does not stop the
--     row being written, so a UNIQUE key here would turn that warning into a
--     failed insert.
--   * there is no code. Identity is the id.

CREATE TABLE IF NOT EXISTS trade_partners (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    partner_type ENUM('CUSTOMER', 'SUPPLIER', 'BOTH') NOT NULL,
    tax_id VARCHAR(32) NULL,
    -- Credit days. 0 is payment on delivery, which is the ordinary case for
    -- retail and the safe default: a term nobody set must not extend credit.
    payment_terms_days INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- The directory is listed by name, which is the order the index stores them
    -- in.
    KEY ix_trade_partners_name (name),
    -- The lookup behind the duplicate warning, and the tax report.
    KEY ix_trade_partners_tax_id (tax_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS trade_partners;
