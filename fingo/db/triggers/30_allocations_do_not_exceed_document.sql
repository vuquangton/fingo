-- A payment cannot settle more than a document owes.
--
-- The domain's PlanAllocation already refuses it, but the domain is not the only
-- thing that can insert an allocation — a repair script or a future bug can, and
-- an over-settled invoice shows as a credit that nobody can explain. See
-- 10_vouchers_no_update_when_posted.sql for why these live here.

CREATE OR REPLACE TRIGGER allocations_do_not_exceed_document
BEFORE INSERT ON allocations
FOR EACH ROW
BEGIN
    DECLARE settled DECIMAL(19, 0) DEFAULT 0;
    DECLARE owed DECIMAL(19, 0) DEFAULT 0;

    SELECT COALESCE(SUM(amount), 0) INTO settled
    FROM allocations WHERE document_id = NEW.document_id;

    SELECT amount INTO owed FROM documents WHERE id = NEW.document_id;

    IF settled + NEW.amount > owed THEN
        SIGNAL SQLSTATE '45000'
        SET MESSAGE_TEXT = 'an allocation would exceed what the document owes';
    END IF;
END;
