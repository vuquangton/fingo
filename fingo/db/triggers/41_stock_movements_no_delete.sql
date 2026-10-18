-- See 40_stock_movements_no_update.sql.

CREATE OR REPLACE TRIGGER stock_movements_no_delete
BEFORE DELETE ON stock_movements
FOR EACH ROW
BEGIN
    SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = 'a stock movement cannot be deleted; record a counter-movement';
END;
