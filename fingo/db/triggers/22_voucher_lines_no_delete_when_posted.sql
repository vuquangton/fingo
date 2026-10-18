-- See 10_vouchers_no_update_when_posted.sql for why this lives here.

CREATE OR REPLACE TRIGGER voucher_lines_no_delete_when_posted
BEFORE DELETE ON voucher_lines
FOR EACH ROW
BEGIN
    IF (SELECT is_posted FROM vouchers WHERE id = OLD.voucher_id) = TRUE THEN
        SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'a line of a posted voucher cannot be deleted';
    END IF;
END;
