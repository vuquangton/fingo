-- A stock movement is a record, not a working copy.
--
-- Together with the replay in the domain, this is what makes the balance a
-- function of history: a movement cannot be edited to make the balance come out
-- differently, and a wrong one is corrected by a counter-movement. See
-- 10_vouchers_no_update_when_posted.sql for why these live here.

CREATE OR REPLACE TRIGGER stock_movements_no_update
BEFORE UPDATE ON stock_movements
FOR EACH ROW
BEGIN
    SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = 'a stock movement cannot be modified; record a counter-movement';
END;
