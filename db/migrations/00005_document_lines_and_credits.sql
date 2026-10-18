-- +goose Up

-- A document's account lines, as they were written. The document header says
-- what it is worth; the lines say how it was built, which is what a VAT return
-- reads (net and tax per line).
CREATE TABLE document_lines (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   uuid NOT NULL REFERENCES clients (id),
    document_id uuid NOT NULL REFERENCES documents (id),
    line_order  integer NOT NULL CHECK (line_order > 0),
    account_code text NOT NULL,
    debit       bigint NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit      bigint NOT NULL DEFAULT 0 CHECK (credit >= 0),
    vat_amount  bigint NOT NULL DEFAULT 0 CHECK (vat_amount >= 0),
    base_amount bigint NOT NULL DEFAULT 0 CHECK (base_amount >= 0),
    CHECK (NOT (debit > 0 AND credit > 0)),
    UNIQUE (document_id, line_order)
);

CREATE INDEX document_lines_document_idx ON document_lines (document_id, line_order);

-- Money a payment left over that the document could not absorb. It is a real
-- position the partner holds, not a rounding difference.
CREATE TABLE document_credits (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   uuid NOT NULL REFERENCES clients (id),
    partner_id  uuid,
    document_id uuid NOT NULL REFERENCES documents (id),
    payment_id  text NOT NULL,
    amount      bigint NOT NULL CHECK (amount > 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX document_credits_partner_idx ON document_credits (partner_id, created_at);

ALTER TABLE document_lines    ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_lines    FORCE  ROW LEVEL SECURITY;
ALTER TABLE document_credits  ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_credits  FORCE  ROW LEVEL SECURITY;

CREATE POLICY document_lines_by_client ON document_lines
    FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client());

CREATE POLICY document_credits_by_client ON document_credits
    FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client());

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON document_lines, document_credits TO fingo_app;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS document_credits;
DROP TABLE IF EXISTS document_lines;
