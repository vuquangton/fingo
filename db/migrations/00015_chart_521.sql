-- +goose Up

-- The seeded TT133 chart has no 521 family, so a sales return had nowhere to go:
-- 5212 "giảm giá hàng bán" and 5213 "hàng bán bị trả lại" were simply absent.
-- A credit note needs 5213.
INSERT INTO chart_template (code, name, parent_code, account_type, nature) VALUES
    ('521',  'Chiết khấu thương mại, giảm giá và hàng bán bị trả lại', NULL, 'REVENUE', 'DEBIT'),
    ('5211', 'Chiết khấu thương mại', '521', 'REVENUE', 'DEBIT'),
    ('5212', 'Giảm giá hàng bán', '521', 'REVENUE', 'DEBIT'),
    ('5213', 'Hàng bán bị trả lại', '521', 'REVENUE', 'DEBIT')
ON CONFLICT (code) DO NOTHING;

-- Existing clients need the same accounts: their chart was copied before this row
-- existed, and the provision function is not re-run for them.
INSERT INTO accounts (client_id, code, name, parent_code, account_type, nature, active)
SELECT c.id, t.code, t.name, t.parent_code, t.account_type, t.nature, true
  FROM clients c
 CROSS JOIN chart_template t
 WHERE t.code IN ('521', '5211', '5212', '5213')
   AND NOT EXISTS (
       SELECT 1 FROM accounts a WHERE a.client_id = c.id AND a.code = t.code);

-- +goose Down

DELETE FROM accounts WHERE code IN ('521', '5211', '5212', '5213');
DELETE FROM chart_template WHERE code IN ('521', '5211', '5212', '5213');
