-- See 50_accounting_period_locks_no_update_when_locked.sql.
--
-- A locked period must not be deleted either. Deleting it would leave the dates
-- it covered belonging to no period at all, which reads as "nothing is locked
-- here" — the opposite of the truth.

CREATE OR REPLACE TRIGGER accounting_period_locks_no_delete_when_locked
BEFORE DELETE ON accounting_period_locks
FOR EACH ROW
BEGIN
    IF OLD.is_locked = TRUE THEN
        SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'a locked accounting period cannot be deleted; locking is irreversible';
    END IF;
END;
