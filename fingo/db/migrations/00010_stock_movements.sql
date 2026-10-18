-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- The stock movement log, and the valuation method each fiscal year is kept by.
--
-- Only movements are stored. A running balance is derived by replaying them —
-- see inventory.Replay — so there is no denormalised total that can drift from
-- the history that produced it, and re-deriving the balance during an audit
-- gives the same answer as the day it was written.

CREATE TABLE IF NOT EXISTS stock_movements (
    id VARCHAR(36) PRIMARY KEY,

    -- The insertion order, and the only sound replay order. Two movements can
    -- share a timestamp to the microsecond; ordering by a random uuid would then
    -- be stable but arbitrary, and an arbitrary order gives a different moving
    -- average.
    seq BIGINT NOT NULL AUTO_INCREMENT,

    item_code VARCHAR(64) NOT NULL,
    warehouse_code VARCHAR(32) NOT NULL,
    kind ENUM('RECEIPT', 'ISSUE') NOT NULL,

    -- Quantities are fractional — kilograms, litres, metres. Amounts are whole
    -- đồng. They are different kinds of number and are stored as such.
    quantity DECIMAL(19, 4) NOT NULL,
    cost DECIMAL(19, 0) NOT NULL,
    voucher_id VARCHAR(36) NULL,
    moved_at DATETIME(6) NOT NULL,
    note VARCHAR(255) NULL,

    UNIQUE KEY uq_stock_movements_seq (seq),
    KEY ix_stock_movements_position (item_code, warehouse_code, seq),
    CONSTRAINT fk_stock_movements_voucher FOREIGN KEY (voucher_id) REFERENCES vouchers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- The method a fiscal year is valued by.
--
-- Per year, not one setting, because TT133 requires the chosen method to be
-- consistent for a whole fiscal year: "Doanh nghiệp lựa chọn phương pháp tính
-- giá nào thì phải đảm bảo tính nhất quán trong cả niên độ kế toán."
CREATE TABLE IF NOT EXISTS inventory_policies (
    fiscal_year SMALLINT PRIMARY KEY,
    method ENUM('SPECIFIC_COST', 'MOVING_AVERAGE', 'PERIODIC_AVERAGE', 'FIFO') NOT NULL,
    chosen_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS inventory_policies;
DROP TABLE IF EXISTS stock_movements;
