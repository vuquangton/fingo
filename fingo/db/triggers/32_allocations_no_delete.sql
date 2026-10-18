-- See 31_allocations_no_update.sql.

CREATE OR REPLACE TRIGGER allocations_no_delete
BEFORE DELETE ON allocations
FOR EACH ROW
BEGIN
    SIGNAL SQLSTATE '45000'
    SET MESSAGE_TEXT = 'an allocation cannot be deleted; reverse the payment and settle again';
END;
