-- A locked accounting period cannot be changed.
--
-- "Irreversible" has to mean irreversible somewhere that survives the domain:
-- the fields are exported, so a caller can already write Locked = false, and a
-- repair script can too. The trigger is the guarantee that outlives both. See
-- 10_vouchers_no_update_when_posted.sql for why these live here.
--
-- Refusing every update rather than only the unlock covers the other ways a
-- locked period could be quietly altered — its dates moved, its timestamp
-- rewritten — each of which would change which transactions are covered.

CREATE OR REPLACE TRIGGER accounting_period_locks_no_update_when_locked
BEFORE UPDATE ON accounting_period_locks
FOR EACH ROW
BEGIN
    IF OLD.is_locked = TRUE THEN
        SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'a locked accounting period cannot be changed; locking is irreversible';
    END IF;
END;
