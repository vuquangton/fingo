-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- Reshape voucher_lines to one account movement per row.
-- See docs/adr/0001-journal-line-model.md.
--
-- The pair shape (debit_account_id + credit_account_id + one shared amount)
-- cannot express an entry with more than one debit or one credit. That is 16
-- of the 18 trading-cycle events, including every VAT sales invoice and every
-- import purchase with landed cost.

ALTER TABLE voucher_lines
    DROP FOREIGN KEY fk_voucher_lines_debit,
    DROP FOREIGN KEY fk_voucher_lines_credit;

ALTER TABLE voucher_lines
    DROP COLUMN debit_account_id,
    DROP COLUMN credit_account_id,
    ADD COLUMN account_id VARCHAR(36) NOT NULL AFTER line_order,
    ADD COLUMN side ENUM('DEBIT', 'CREDIT') NOT NULL AFTER account_id,
    MODIFY COLUMN amount DECIMAL(19, 0) NOT NULL DEFAULT 0;

ALTER TABLE voucher_lines
    ADD CONSTRAINT fk_voucher_lines_account FOREIGN KEY (account_id) REFERENCES accounts(id);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

ALTER TABLE voucher_lines
    DROP FOREIGN KEY fk_voucher_lines_account;

ALTER TABLE voucher_lines
    DROP COLUMN side,
    DROP COLUMN account_id,
    ADD COLUMN debit_account_id VARCHAR(36) NOT NULL AFTER line_order,
    ADD COLUMN credit_account_id VARCHAR(36) NOT NULL AFTER debit_account_id,
    MODIFY COLUMN amount DECIMAL(18, 4) NOT NULL DEFAULT 0.0000;

ALTER TABLE voucher_lines
    ADD CONSTRAINT fk_voucher_lines_debit FOREIGN KEY (debit_account_id) REFERENCES accounts(id),
    ADD CONSTRAINT fk_voucher_lines_credit FOREIGN KEY (credit_account_id) REFERENCES accounts(id);
