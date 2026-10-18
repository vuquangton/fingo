-- A posted voucher is a record, not a working copy.
--
-- The domain already refuses to change one, but the domain is not the only thing
-- that can reach these tables: a repair script, a future migration, or a bug in
-- an adapter all write SQL directly. The trigger is the guarantee that survives
-- all of them, which is why it lives in the database rather than in Go.
--
-- These live outside db/migrations because sqlc's MySQL parser cannot read
-- CREATE TRIGGER, and the alternative — listing schema files in sqlc.yaml by
-- hand — would let the generated models drift silently the first time someone
-- added a migration and forgot. They are re-applied on every open instead, which
-- also means an accidentally dropped trigger restores itself.

CREATE OR REPLACE TRIGGER vouchers_no_update_when_posted
BEFORE UPDATE ON vouchers
FOR EACH ROW
BEGIN
    IF OLD.is_posted = TRUE THEN
        SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'a posted voucher cannot be modified; reverse it and post a counter-entry';
    END IF;
END;
