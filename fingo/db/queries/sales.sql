-- name: CreateSalesInvoice :exec
INSERT INTO sales_invoices (
    id,
    voucher_id,
    company_profile_id,
    customer_id,
    invoice_template,
    invoice_series,
    invoice_no,
    invoice_date,
    due_date,
    payment_method,
    einvoice_status,
    einvoice_code_cqt,
    subtotal_vnd,
    discount_vnd,
    vat_amount_vnd,
    total_amount_vnd,
    is_stock_outward_auto
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: CreateSalesInvoiceLine :exec
INSERT INTO sales_invoice_lines (
    id,
    sales_invoice_id,
    line_order,
    item_id,
    warehouse_id,
    debit_account_id,
    credit_account_id,
    quantity,
    unit_price_vnd,
    amount_vnd,
    discount_rate,
    discount_amount_vnd,
    vat_rate,
    vat_amount_vnd,
    note
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetSalesInvoiceByID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    customer_id,
    invoice_template,
    invoice_series,
    invoice_no,
    invoice_date,
    due_date,
    payment_method,
    einvoice_status,
    einvoice_code_cqt,
    subtotal_vnd,
    discount_vnd,
    vat_amount_vnd,
    total_amount_vnd,
    is_stock_outward_auto
FROM sales_invoices
WHERE id = ?;

-- name: GetSalesInvoiceByVoucherID :one
SELECT 
    id,
    voucher_id,
    company_profile_id,
    customer_id,
    invoice_template,
    invoice_series,
    invoice_no,
    invoice_date,
    due_date,
    payment_method,
    einvoice_status,
    einvoice_code_cqt,
    subtotal_vnd,
    discount_vnd,
    vat_amount_vnd,
    total_amount_vnd,
    is_stock_outward_auto
FROM sales_invoices
WHERE voucher_id = ?;

-- name: ListSalesInvoiceLinesByInvoiceID :many
SELECT 
    id,
    sales_invoice_id,
    line_order,
    item_id,
    warehouse_id,
    debit_account_id,
    credit_account_id,
    quantity,
    unit_price_vnd,
    amount_vnd,
    discount_rate,
    discount_amount_vnd,
    vat_rate,
    vat_amount_vnd,
    note
FROM sales_invoice_lines
WHERE sales_invoice_id = ?
ORDER BY line_order ASC;

-- name: UpdateSalesInvoiceEInvoiceStatus :exec
UPDATE sales_invoices
SET einvoice_status = ?, einvoice_code_cqt = ?
WHERE id = ?;
