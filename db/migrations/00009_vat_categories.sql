-- +goose Up

-- A VAT category, distinct from a rate.
--
-- 0% and exempt (KCT) both post no output VAT, but they are different lines on a
-- return and different facts about the sale. Without this column a practice must
-- record an exempt invoice as 0%, which misstates the return. The rate row keeps
-- the arithmetic; the category keeps the meaning.
ALTER TABLE vat_rates
    ADD COLUMN category text NOT NULL DEFAULT 'TAXABLE'
    CHECK (category IN ('TAXABLE', 'KCT', 'EXEMPT'));

INSERT INTO vat_rates (id, name, basis_points, effective_from, category) VALUES
    ('kct',    'KCT — không chịu thuế GTGT',              0, DATE '2026-01-01', 'KCT'),
    ('exempt', 'Miễn thuế (không kê khai)',               0, DATE '2026-01-01', 'EXEMPT');

-- +goose Down

DELETE FROM vat_rates WHERE id IN ('kct', 'exempt');
ALTER TABLE vat_rates DROP COLUMN category;
