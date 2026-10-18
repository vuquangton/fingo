-- +goose Up

-- Customers and suppliers. Sales cannot post without one: the receivable line
-- names who owes it, and TT133 keeps Sổ chi tiết TK 131 by customer.
CREATE TABLE partners (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  uuid NOT NULL REFERENCES clients (id),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    tax_id     text NOT NULL DEFAULT '',
    kind       text NOT NULL CHECK (kind IN ('CUSTOMER', 'SUPPLIER', 'BOTH')),
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (client_id, name)
);

-- A tax code identifies a taxpayer, so the same code cannot name two partners.
-- An empty code is allowed: a walk-in customer has none, and the partial index
-- keeps them out of the uniqueness rule.
CREATE UNIQUE INDEX partners_tax_id_key
    ON partners (client_id, tax_id)
    WHERE tax_id <> '';

ALTER TABLE partners ENABLE ROW LEVEL SECURITY;
ALTER TABLE partners FORCE ROW LEVEL SECURITY;

CREATE POLICY partners_by_client ON partners
    FOR ALL
    USING (client_id = app_current_client())
    WITH CHECK (client_id = app_current_client());

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON partners TO fingo_app;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS partners;
