-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- A submitted request posts exactly once, however many times it arrives.
--
-- FinGo is a desktop app, but a double-clicked button, a retried Wails call, or
-- an operator repeating a step they were unsure about all deliver the same
-- request twice. Without this, a duplicate sale is a duplicate invoice, a
-- duplicate VAT liability and a duplicate receivable — and nothing downstream
-- can tell it apart from a real second sale.
--
-- The key is recorded in the same transaction as the voucher it guards, so there
-- is no window in which one exists without the other.

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key_value VARCHAR(128) NOT NULL,
    voucher_id VARCHAR(36) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (key_value),
    CONSTRAINT fk_idempotency_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS idempotency_keys;
