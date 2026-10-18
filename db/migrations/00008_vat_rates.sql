-- +goose Up

-- The VAT rate catalog, as data rather than as code.
--
-- Rates are national, not per client, so this table has no client_id and no RLS.
-- A line names the row it was taxed under; the rate itself is read back, so a
-- stored line's tax stays reproducible when the table gains a row later.
--
-- The 8% reduced window is deliberately NOT seeded: its statutory dates are not
-- confirmed in docs/legal-basis.md. Add it as data once the practitioner confirms
-- the window; an absent rate refuses the entry rather than taxing it wrongly.
CREATE TABLE vat_rates (
    id             text PRIMARY KEY CHECK (btrim(id) <> ''),
    name           text NOT NULL CHECK (btrim(name) <> ''),
    basis_points   integer NOT NULL CHECK (basis_points BETWEEN 0 AND 10000),
    effective_from date NOT NULL,
    effective_to   date,
    active         boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

INSERT INTO vat_rates (id, name, basis_points, effective_from) VALUES
    ('vat0',  '0% — hàng hoá, dịch vụ chịu thuế 0%', 0,    DATE '2026-01-01'),
    ('vat5',  '5% — hàng hoá, dịch vụ chịu thuế 5%', 500,  DATE '2026-01-01'),
    ('vat10', '10% — thuế suất phổ thông',           1000, DATE '2026-01-01');

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT SELECT, INSERT, UPDATE ON vat_rates TO fingo_app;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS vat_rates;
