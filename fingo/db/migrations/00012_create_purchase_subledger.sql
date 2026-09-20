-- +goose Up
CREATE TABLE IF NOT EXISTS purchase_invoices (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    vendor_id VARCHAR(36) NOT NULL,
    invoice_template VARCHAR(20) NOT NULL DEFAULT '1',
    invoice_series VARCHAR(10) NOT NULL,
    invoice_no VARCHAR(20) NOT NULL,
    invoice_date DATE NOT NULL,
    due_date DATE NOT NULL,
    payment_status ENUM('UNPAID', 'PARTIALLY_PAID', 'PAID') NOT NULL DEFAULT 'UNPAID',
    subtotal_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    paid_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    is_stock_inward_auto BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT fk_pi_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_pi_vendor FOREIGN KEY (vendor_id) REFERENCES vendors (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS purchase_invoice_lines (
    id VARCHAR(36) PRIMARY KEY,
    purchase_invoice_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    warehouse_id VARCHAR(36) NULL,
    debit_account_id VARCHAR(36) NOT NULL,
    credit_account_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL,
    unit_price_vnd DECIMAL(18, 0) NOT NULL,
    amount_vnd DECIMAL(18, 0) NOT NULL,
    vat_rate DECIMAL(5, 2) NOT NULL DEFAULT 0.00,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    note VARCHAR(255) NULL,
    CONSTRAINT fk_pil_invoice FOREIGN KEY (purchase_invoice_id) REFERENCES purchase_invoices (id) ON DELETE CASCADE,
    CONSTRAINT fk_pil_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS purchase_invoice_lines;
DROP TABLE IF EXISTS purchase_invoices;
