-- name: CreatePurchaseInvoice :exec
INSERT INTO purchase_invoices (
    id,
    voucher_id,
    company_profile_id,
    vendor_id,
    invoice_template,
    invoice_series,
    invoice_no,
    invoice_date,
    due_date,
    payment_status,
    subtotal_vnd,
    vat_amount_vnd,
    total_amount_vnd,
    paid_amount_vnd,
    is_stock_inward_auto
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: CreatePurchaseInvoiceLine :exec
INSERT INTO purchase_invoice_lines (
    id,
    purchase_invoice_id,
    line_order,
    item_id,
    warehouse_id,
    debit_account_id,
    credit_account_id,
    quantity,
    unit_price_vnd,
    amount_vnd,
    vat_rate,
    vat_amount_vnd,
    note
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetPurchaseInvoiceByID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    vendor_id,
    invoice_template,
    invoice_series,
    invoice_no,
    invoice_date,
    due_date,
    payment_status,
    subtotal_vnd,
    vat_amount_vnd,
    total_amount_vnd,
    paid_amount_vnd,
    is_stock_inward_auto
FROM purchase_invoices
WHERE id = ?;

-- name: GetPurchaseInvoiceByVoucherID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    vendor_id,
    invoice_template,
    invoice_series,
    invoice_no,
    invoice_date,
    due_date,
    payment_status,
    subtotal_vnd,
    vat_amount_vnd,
    total_amount_vnd,
    paid_amount_vnd,
    is_stock_inward_auto
FROM purchase_invoices
WHERE voucher_id = ?;

-- name: ListPurchaseInvoiceLinesByInvoiceID :many
SELECT 
    id,
    purchase_invoice_id,
    line_order,
    item_id,
    warehouse_id,
    debit_account_id,
    credit_account_id,
    quantity,
    unit_price_vnd,
    amount_vnd,
    vat_rate,
    vat_amount_vnd,
    note
FROM purchase_invoice_lines
WHERE purchase_invoice_id = ?
ORDER BY line_order ASC;

-- name: UpdatePurchaseInvoicePaymentStatus :exec
UPDATE purchase_invoices
SET payment_status = ?, paid_amount_vnd = ?
WHERE id = ?;
