-- +goose Up

-- Stocked items. A movement names an item; the item names what it is counted in.
CREATE TABLE items (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id     uuid NOT NULL REFERENCES clients (id),
    sku           text NOT NULL CHECK (btrim(sku) <> ''),
    name          text NOT NULL CHECK (btrim(name) <> ''),
    unit          text NOT NULL CHECK (btrim(unit) <> ''),
    reorder_point integer NOT NULL DEFAULT 0 CHECK (reorder_point >= 0),
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (client_id, sku)
);

ALTER TABLE items ENABLE ROW LEVEL SECURITY;
ALTER TABLE items FORCE ROW LEVEL SECURITY;

CREATE POLICY items_by_client ON items
    FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client());

-- The movement log already exists; it needs the two facts the inventory engine
-- records per movement: what one unit cost, and which document it came from.
ALTER TABLE stock_movements
    ADD COLUMN unit_cost     bigint NOT NULL DEFAULT 0 CHECK (unit_cost >= 0),
    ADD COLUMN source_doc_id text   NOT NULL DEFAULT '';

CREATE INDEX stock_movements_client_seq_idx ON stock_movements (client_id, seq);

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON items TO fingo_app;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down

DROP INDEX IF EXISTS stock_movements_client_seq_idx;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS source_doc_id;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS unit_cost;
DROP TABLE IF EXISTS items;
