-- +goose Up

-- What a sale actually sold, line by line.
--
-- The ledger keeps accounts and amounts; a credit note needs the item, the
-- quantity, the price and the rate that produced those amounts, or a return cannot
-- be calculated at all. cost_at_sale is the moving-average cost at the moment the
-- goods left, so a return restores stock at the cost it was issued at.
CREATE TABLE sale_lines (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id    uuid NOT NULL REFERENCES clients (id),
    document_id  uuid NOT NULL REFERENCES documents (id),
    line_no      integer NOT NULL CHECK (line_no > 0),
    item_id      text NOT NULL DEFAULT '',
    quantity     integer NOT NULL CHECK (quantity > 0),
    unit_price   bigint NOT NULL CHECK (unit_price >= 0),
    vat_rate_id  text REFERENCES vat_rates (id),
    cost_at_sale bigint NOT NULL DEFAULT 0 CHECK (cost_at_sale >= 0),
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, line_no)
);

ALTER TABLE sale_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE sale_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY sale_lines_by_client ON sale_lines
    FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client());

-- A credit note names the invoice it credits.
ALTER TABLE documents ADD COLUMN source_document_id uuid REFERENCES documents (id);
CREATE INDEX documents_source_idx ON documents (source_document_id);

GRANT SELECT, INSERT ON sale_lines TO fingo_app;

-- +goose Down

ALTER TABLE documents DROP COLUMN source_document_id;
DROP TABLE IF EXISTS sale_lines;