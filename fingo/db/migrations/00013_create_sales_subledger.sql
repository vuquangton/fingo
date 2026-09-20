-- +goose Up
CREATE TABLE IF NOT EXISTS sales_invoices (
    id VARCHAR(36) PRIMARY KEY,
    voucher_id VARCHAR(36) NOT NULL UNIQUE,
    company_profile_id VARCHAR(36) NOT NULL,
    customer_id VARCHAR(36) NOT NULL,
    invoice_template VARCHAR(20) NOT NULL DEFAULT '1',
    invoice_series VARCHAR(10) NOT NULL,
    invoice_no VARCHAR(20) NOT NULL,
    invoice_date DATE NOT NULL,
    due_date DATE NOT NULL,
    payment_method VARCHAR(20) NOT NULL DEFAULT 'CK',
    einvoice_status ENUM('DRAFT', 'SIGNED', 'CQT_SENT', 'CQT_ACCEPTED', 'CQT_REJECTED') NOT NULL DEFAULT 'DRAFT',
    einvoice_code_cqt VARCHAR(50) NULL,
    subtotal_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    discount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    total_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    is_stock_outward_auto BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT fk_si_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers (id) ON DELETE RESTRICT,
    CONSTRAINT fk_si_customer FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sales_invoice_lines (
    id VARCHAR(36) PRIMARY KEY,
    sales_invoice_id VARCHAR(36) NOT NULL,
    line_order INT NOT NULL,
    item_id VARCHAR(36) NOT NULL,
    warehouse_id VARCHAR(36) NULL,
    debit_account_id VARCHAR(36) NOT NULL,
    credit_account_id VARCHAR(36) NOT NULL,
    quantity DECIMAL(18, 4) NOT NULL,
    unit_price_vnd DECIMAL(18, 0) NOT NULL,
    amount_vnd DECIMAL(18, 0) NOT NULL,
    discount_rate DECIMAL(5, 2) NOT NULL DEFAULT 0.00,
    discount_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    vat_rate DECIMAL(5, 2) NOT NULL DEFAULT 0.00,
    vat_amount_vnd DECIMAL(18, 0) NOT NULL DEFAULT 0,
    note VARCHAR(255) NULL,
    CONSTRAINT fk_sil_invoice FOREIGN KEY (sales_invoice_id) REFERENCES sales_invoices (id) ON DELETE CASCADE,
    CONSTRAINT fk_sil_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
DROP TABLE IF EXISTS sales_invoice_lines;
DROP TABLE IF EXISTS sales_invoices;
