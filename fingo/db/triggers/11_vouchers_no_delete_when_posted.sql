-- See 10_vouchers_no_update_when_posted.sql for why this lives here.

CREATE OR REPLACE TRIGGER vouchers_no_delete_when_posted
BEFORE DELETE ON vouchers
FOR EACH ROW
BEGIN
    IF OLD.is_posted = TRUE THEN
        SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'a posted voucher cannot be deleted; reverse it and post a counter-entry';
    END IF;
END;
