-- An allocation is a record, not a working copy.
--
-- Correcting a mis-allocation means reversing the payment and settling again,
-- not rewriting history. See 10_vouchers_no_update_when_posted.sql for why these
-- live here.

CREATE OR REPLACE TRIGGER allocations_no_update
BEFORE UPDATE ON allocations
FOR EACH ROW
BEGIN
    SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = 'an allocation cannot be modified; reverse the payment and settle again';
END;
