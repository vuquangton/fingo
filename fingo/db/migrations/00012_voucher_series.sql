-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- One run of voucher numbers per prefix per fiscal year.
--
-- NOTE: this coexists with voucher_sequences (00005), which serves the older
-- model keyed by book. Two tables answering "what comes next" is one too many;
-- consolidating them is a separate decision and is not made here.

CREATE TABLE IF NOT EXISTS voucher_series (
    id VARCHAR(36) PRIMARY KEY,
    prefix VARCHAR(16) NOT NULL,
    fiscal_year SMALLINT NOT NULL,
    last_number INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- A series is one prefix in one year. This key is what makes two callers
    -- starting the same run collapse into one series rather than two, and it is
    -- what the allocation statement collides on.
    UNIQUE KEY uq_voucher_series (prefix, fiscal_year)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS voucher_series;
