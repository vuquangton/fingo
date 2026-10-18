-- +goose Up

-- The VAT rate a document was taxed under. Without it the books can say how much
-- 33311 moved but not why, and a return or a query cannot separate 10% turnover
-- from 5%, KCT or exempt. Nullable: documents posted before this migration did not
-- record it, and inventing a rate for them would be a fiction.
ALTER TABLE documents ADD COLUMN vat_rate_id text REFERENCES vat_rates (id);

CREATE INDEX documents_vat_rate_idx ON documents (vat_rate_id, document_date);

-- +goose Down

DROP INDEX IF EXISTS documents_vat_rate_idx;
ALTER TABLE documents DROP COLUMN vat_rate_id;