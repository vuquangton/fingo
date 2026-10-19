-- +goose Up

-- Reversing a payment must un-apply it, but `allocations` and `document_credits`
-- are append-only: a correct system records the reversal rather than deleting the
-- history of the application.
--
-- Effective allocated = SUM(allocations.amount) - SUM(allocation_reversals.amount).
-- The original rows stay, and the reversal is itself an append-only fact.
CREATE TABLE allocation_reversals (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id     uuid NOT NULL REFERENCES clients (id),
    allocation_id uuid NOT NULL REFERENCES allocations (id),
    voucher_id    uuid NOT NULL REFERENCES vouchers (id),
    amount        bigint NOT NULL CHECK (amount > 0),
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (allocation_id)
);

CREATE TABLE document_credit_reversals (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   uuid NOT NULL REFERENCES clients (id),
    credit_id   uuid NOT NULL REFERENCES document_credits (id),
    voucher_id  uuid NOT NULL REFERENCES vouchers (id),
    amount      bigint NOT NULL CHECK (amount > 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (credit_id)
);

ALTER TABLE allocation_reversals ENABLE ROW LEVEL SECURITY;
ALTER TABLE allocation_reversals FORCE ROW LEVEL SECURITY;
CREATE POLICY allocation_reversals_by_client ON allocation_reversals
    FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client());

ALTER TABLE document_credit_reversals ENABLE ROW LEVEL SECURITY;
ALTER TABLE document_credit_reversals FORCE ROW LEVEL SECURITY;
CREATE POLICY document_credit_reversals_by_client ON document_credit_reversals
    FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client());

CREATE TRIGGER allocation_reversals_append_only BEFORE UPDATE OR DELETE ON allocation_reversals
    FOR EACH ROW EXECUTE FUNCTION append_only();
CREATE TRIGGER document_credit_reversals_append_only BEFORE UPDATE OR DELETE ON document_credit_reversals
    FOR EACH ROW EXECUTE FUNCTION append_only();

CREATE INDEX allocation_reversals_voucher_idx ON allocation_reversals (client_id, voucher_id);

GRANT SELECT, INSERT ON allocation_reversals TO fingo_app;
GRANT SELECT, INSERT ON document_credit_reversals TO fingo_app;

-- +goose Down

DROP TABLE IF EXISTS document_credit_reversals;
DROP TABLE IF EXISTS allocation_reversals;
