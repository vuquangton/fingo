-- +goose Up

-- A register of statutory VAT windows the catalog cannot yet express.
--
-- It holds no dates by default: a window is entered only when a practitioner
-- confirms the dates, because inventing them would tax real invoices at the wrong
-- rate. While a window is active and covers an invoice date, the sales screen warns
-- and requires an explicit acknowledgement, which is written to the audit log.
CREATE TABLE vat_unresolved_windows (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rate_id    text NOT NULL REFERENCES vat_rates (id),
    from_date  date NOT NULL,
    to_date    date NOT NULL,
    note       text NOT NULL DEFAULT '',
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (to_date >= from_date)
);

GRANT SELECT, INSERT, UPDATE ON vat_unresolved_windows TO fingo_app;

-- +goose Down

DROP TABLE IF EXISTS vat_unresolved_windows;
