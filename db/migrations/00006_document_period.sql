-- +goose Up

-- A document must name the period it falls in: settlement validates it, and a
-- document nobody can place in a period cannot be reported. The column is
-- backfilled from the range each existing document's date falls in.
ALTER TABLE documents ADD COLUMN period_id uuid REFERENCES accounting_periods (id);

UPDATE documents d
   SET period_id = p.id
  FROM accounting_periods p
 WHERE d.period_id IS NULL
   AND d.document_date BETWEEN p.start_date AND p.end_date;

CREATE INDEX documents_period_idx ON documents (period_id, document_date);

-- +goose Down

DROP INDEX IF EXISTS documents_period_idx;
ALTER TABLE documents DROP COLUMN period_id;