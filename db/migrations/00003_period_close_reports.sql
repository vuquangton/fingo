-- +goose Up

-- The report a close recorded, so a period that was closed can state how it
-- closed. Stored as JSON: the report is a snapshot of a finished computation, not
-- a set of rows anything joins to.
CREATE TABLE period_close_reports (
    period_id  uuid PRIMARY KEY REFERENCES accounting_periods (id),
    client_id  uuid NOT NULL REFERENCES clients (id),
    report     jsonb NOT NULL,
    closed_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE period_close_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE period_close_reports FORCE ROW LEVEL SECURITY;

CREATE POLICY period_close_reports_by_client ON period_close_reports
    FOR ALL
    USING (client_id = app_current_client())
    WITH CHECK (client_id = app_current_client());

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON period_close_reports TO fingo_app;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS period_close_reports;
