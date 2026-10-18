-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

-- The AR/AP document book: documents, the lines that make them up, what has
-- settled them, and the credits an overpayment leaves behind.
--
-- NOTE: this coexists with `documents` and `allocations` (00009), which carry a
-- voucher_id, a reference number and a due date, and are what the posting path
-- currently writes. They are two models of the same idea and one of them has to
-- go — this one tracks how much of a document has been settled and in what
-- state, which the older one leaves to be recomputed from its allocations on
-- every read. That consolidation is a separate decision and is not made here.
-- The names are deliberately distinct so nothing in this migration can clobber
-- the existing tables.
--
-- Three things about the shape are load-bearing:
--
--   * allocated_amount and status are stored, not derived. They are one fact
--     written together under a row lock, which is what lets a report scan by
--     status without summing every allocation, and what makes the status the
--     same on every read rather than a function of how much of the history that
--     read happened to see.
--   * amounts are BIGINT minor units beside a CHAR(3) currency, matching
--     internal/money. There is no dividing by a hundred anywhere, and no float.
--   * partner_id, period_id and vat_rate_id are indexed but not foreign keys.
--     partner_id would have to point at one of the two partner models that are
--     awaiting consolidation, period_id at one of the two period models, and
--     vat_rate_id at a rate table that does not exist yet — internal/vat holds
--     its registry in memory. Pinning any of them now would decide a question
--     that has not been answered.

CREATE TABLE IF NOT EXISTS arap_documents (
    id VARCHAR(36) PRIMARY KEY,
    doc_type ENUM('INVOICE', 'CREDIT_NOTE', 'RECEIPT', 'PAYMENT', 'JOURNAL') NOT NULL,
    partner_id VARCHAR(36) NOT NULL,
    document_date DATE NOT NULL,
    period_id VARCHAR(36) NOT NULL,
    vat_rate_id VARCHAR(36) NULL,
    total_amount BIGINT NOT NULL,
    allocated_amount BIGINT NOT NULL DEFAULT 0,
    currency CHAR(3) NOT NULL,
    status ENUM('OPEN', 'PARTIALLY_PAID', 'PAID', 'VOID') NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Aging reads a partner's documents oldest first, which is the order the
    -- index stores them in.
    KEY ix_arap_documents_partner (partner_id, document_date),
    -- The open-item list and the period close read by status.
    KEY ix_arap_documents_status (status, document_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- The account lines a document is made of. Amounts are in the document's
-- currency, which is why this table does not repeat it.
CREATE TABLE IF NOT EXISTS arap_lines (
    document_id VARCHAR(36) NOT NULL,
    line_no INT NOT NULL,
    account_id VARCHAR(36) NOT NULL,
    debit BIGINT NOT NULL DEFAULT 0,
    credit BIGINT NOT NULL DEFAULT 0,
    vat_amount BIGINT NOT NULL DEFAULT 0,
    base_amount BIGINT NOT NULL DEFAULT 0,

    -- The lines of one document are a sequence, and (document, line_no) is what
    -- identifies one — a line has no identity of its own to name.
    PRIMARY KEY (document_id, line_no),
    CONSTRAINT fk_arap_lines_document FOREIGN KEY (document_id) REFERENCES arap_documents(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- One payment applied to one document.
CREATE TABLE IF NOT EXISTS arap_allocations (
    id VARCHAR(36) PRIMARY KEY,
    document_id VARCHAR(36) NOT NULL,
    payment_id VARCHAR(36) NOT NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    allocated_at DATETIME(6) NOT NULL,

    -- A payment applies to a given document once. This is the second line of
    -- defence behind the repository's check: two concurrent applications of the
    -- same payment cannot both write, whatever the application code does.
    UNIQUE KEY uq_arap_allocations_payment (document_id, payment_id),
    KEY ix_arap_allocations_document (document_id, allocated_at),
    CONSTRAINT fk_arap_allocations_document FOREIGN KEY (document_id) REFERENCES arap_documents(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Money a payment left over that the document could not absorb.
CREATE TABLE IF NOT EXISTS arap_credits (
    id VARCHAR(36) PRIMARY KEY,
    partner_id VARCHAR(36) NOT NULL,
    document_id VARCHAR(36) NOT NULL,
    payment_id VARCHAR(36) NOT NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    created_at DATETIME(6) NOT NULL,

    -- An overpayment is recorded once, for the same reason an allocation is.
    UNIQUE KEY uq_arap_credits_payment (document_id, payment_id),
    KEY ix_arap_credits_partner (partner_id, created_at),
    CONSTRAINT fk_arap_credits_document FOREIGN KEY (document_id) REFERENCES arap_documents(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back

DROP TABLE IF EXISTS arap_credits;
DROP TABLE IF EXISTS arap_allocations;
DROP TABLE IF EXISTS arap_lines;
DROP TABLE IF EXISTS arap_documents;
