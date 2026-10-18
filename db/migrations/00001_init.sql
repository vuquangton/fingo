-- +goose Up

-- FinGo v2 initial schema.
--
-- One database, every business row owned by a client, enforced by row-level
-- security rather than by convention (ADR 0006). Money is bigint đồng (ADR
-- 0008). Rules that a future caller must not be able to bypass live here as
-- constraints and triggers, not in Go.
--
-- Seeding: the TT133 chart is generated from the engine's own table
-- (tools/genchart) so the migration and gl.TT133Chart() cannot drift.

-- ---------------------------------------------------------------------------
-- tenancy registry (no RLS: it holds no financial data)
-- ---------------------------------------------------------------------------

CREATE TABLE clients (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    tax_id     text NOT NULL UNIQUE CHECK (btrim(tax_id) <> ''),
    active     boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- The statutory chart, shared by every client. A client's accounts are a copy,
-- made when the client is provisioned, so a later statutory change is a
-- migration that touches this table plus a re-seed, never a mutation of a
-- client's posted history.
CREATE TABLE chart_template (
    code         text PRIMARY KEY,
    name         text NOT NULL,
    parent_code  text REFERENCES chart_template (code),
    account_type text NOT NULL CHECK (account_type IN ('ASSET','LIABILITY','EQUITY','REVENUE','EXPENSE')),
    nature       text NOT NULL CHECK (nature IN ('DEBIT','CREDIT','DUAL'))
);

-- ---------------------------------------------------------------------------
-- practice-level tables (no RLS: administrative, no client financial data)
-- ---------------------------------------------------------------------------

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE CHECK (btrim(email) <> ''),
    display_name  text NOT NULL,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('admin','accountant','viewer')),
    active        boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz
);

-- Append-only: a trigger below refuses UPDATE and DELETE.
CREATE TABLE audit_log (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id  uuid NOT NULL REFERENCES clients (id),
    at         timestamptz NOT NULL DEFAULT now(),
    actor      text NOT NULL,
    action     text NOT NULL,
    entity     text NOT NULL,
    entity_id  text NOT NULL,
    before     jsonb,
    after      jsonb
);

-- ---------------------------------------------------------------------------
-- client-scoped tables
-- ---------------------------------------------------------------------------

CREATE TABLE accounts (
    client_id    uuid NOT NULL REFERENCES clients (id),
    code         text NOT NULL,
    name         text NOT NULL,
    parent_code  text,
    account_type text NOT NULL CHECK (account_type IN ('ASSET','LIABILITY','EQUITY','REVENUE','EXPENSE')),
    nature       text NOT NULL CHECK (nature IN ('DEBIT','CREDIT','DUAL')),
    active       boolean NOT NULL DEFAULT true,
    PRIMARY KEY (client_id, code),
    FOREIGN KEY (client_id, parent_code) REFERENCES accounts (client_id, code)
);

CREATE TABLE accounting_periods (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  uuid NOT NULL REFERENCES clients (id),
    name       text NOT NULL,
    start_date date NOT NULL,
    end_date   date NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (start_date <= end_date),
    UNIQUE (client_id, start_date, end_date)
);

-- Irreversible by trigger: no UPDATE, no DELETE.
CREATE TABLE accounting_period_locks (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  uuid NOT NULL REFERENCES clients (id),
    period_id  uuid NOT NULL UNIQUE REFERENCES accounting_periods (id),
    locked_at  timestamptz NOT NULL DEFAULT now(),
    locked_by  text NOT NULL,
    reason     text NOT NULL DEFAULT ''
);

CREATE TABLE vouchers (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id    uuid NOT NULL REFERENCES clients (id),
    voucher_no   text NOT NULL,
    book         text NOT NULL DEFAULT 'GENERAL',
    fiscal_year  integer NOT NULL CHECK (fiscal_year BETWEEN 1900 AND 2999),
    seq          integer NOT NULL CHECK (seq > 0),
    voucher_date date NOT NULL,
    description  text NOT NULL DEFAULT '',
    posted_at    timestamptz,
    posted_by    text,
    created_by   text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (client_id, book, fiscal_year, seq),
    UNIQUE (client_id, voucher_no)
);

CREATE TABLE voucher_lines (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id    uuid NOT NULL REFERENCES clients (id),
    voucher_id   uuid NOT NULL REFERENCES vouchers (id),
    line_order   integer NOT NULL CHECK (line_order > 0),
    account_code text NOT NULL,
    side         text NOT NULL CHECK (side IN ('DEBIT','CREDIT')),
    amount       bigint NOT NULL CHECK (amount > 0),
    note         text NOT NULL DEFAULT '',
    UNIQUE (voucher_id, line_order),
    FOREIGN KEY (client_id, account_code) REFERENCES accounts (client_id, code)
);

CREATE TABLE documents (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id     uuid NOT NULL REFERENCES clients (id),
    kind          text NOT NULL CHECK (kind IN ('SALES_INVOICE','CREDIT_NOTE','PURCHASE_BILL','PAYMENT','RECEIPT')),
    partner_id    uuid,
    document_date date NOT NULL,
    due_date      date,
    total         bigint NOT NULL CHECK (total >= 0),
    allocated     bigint NOT NULL DEFAULT 0 CHECK (allocated >= 0),
    status        text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','PARTIAL','SETTLED','VOID')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    CHECK (allocated <= total),
    CHECK (due_date IS NULL OR due_date >= document_date)
);

CREATE TABLE allocations (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   uuid NOT NULL REFERENCES clients (id),
    document_id uuid NOT NULL REFERENCES documents (id),
    voucher_id  uuid NOT NULL REFERENCES vouchers (id),
    amount      bigint NOT NULL CHECK (amount > 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (document_id, voucher_id)
);

CREATE TABLE inventory_policies (
    client_id   uuid NOT NULL REFERENCES clients (id),
    fiscal_year integer NOT NULL,
    method      text NOT NULL CHECK (method IN ('SPECIFIC_COST','MOVING_AVERAGE','PERIODIC_AVERAGE','FIFO')),
    chosen_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (client_id, fiscal_year)
);

CREATE TABLE stock_movements (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id      uuid NOT NULL REFERENCES clients (id),
    seq            bigint GENERATED ALWAYS AS IDENTITY,
    item_code      text NOT NULL,
    warehouse_code text NOT NULL,
    kind           text NOT NULL CHECK (kind IN ('RECEIPT','ISSUE')),
    quantity       numeric(19,4) NOT NULL CHECK (quantity <> 0),
    cost           bigint NOT NULL CHECK (cost >= 0),
    voucher_id     uuid REFERENCES vouchers (id),
    moved_at       timestamptz NOT NULL DEFAULT now(),
    note           text NOT NULL DEFAULT '',
    UNIQUE (seq)
);

CREATE INDEX voucher_lines_voucher_idx  ON voucher_lines (voucher_id, line_order);
CREATE INDEX vouchers_client_date_idx   ON vouchers (client_id, voucher_date);
CREATE INDEX documents_client_kind_idx  ON documents (client_id, kind, document_date);
CREATE INDEX stock_movements_item_idx   ON stock_movements (client_id, item_code, seq);

-- +goose StatementBegin

-- The current client, or NULL. The two-argument current_setting is deliberate:
-- the one-argument form raises when the setting is absent, which would turn a
-- forgotten scope into a 500 instead of zero rows.
CREATE FUNCTION app_current_client() RETURNS uuid
LANGUAGE sql STABLE AS $$
    SELECT nullif(current_setting('app.client_id', true), '')::uuid
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION app_require_client() RETURNS uuid
LANGUAGE plpgsql STABLE AS $$
DECLARE
    v uuid := app_current_client();
BEGIN
    IF v IS NULL THEN
        RAISE EXCEPTION 'no app.client_id set: refusing an unscoped write'
            USING ERRCODE = 'insufficient_privilege';
    END IF;
    RETURN v;
END
$$;

-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- rules that must survive a caller: immutability, balance, allocation, locks
-- ---------------------------------------------------------------------------

-- +goose StatementBegin

CREATE FUNCTION vouchers_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.posted_at IS NOT NULL THEN
            RAISE EXCEPTION 'posted voucher % cannot be deleted', OLD.voucher_no
                USING ERRCODE = 'integrity_constraint_violation';
        END IF;
        RETURN OLD;
    END IF;

    IF OLD.posted_at IS NOT NULL THEN
        RAISE EXCEPTION 'posted voucher % is immutable', OLD.voucher_no
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;

    IF NEW.client_id <> OLD.client_id THEN
        RAISE EXCEPTION 'a voucher cannot change client'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;

    RETURN NEW;
END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION voucher_lines_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_id      uuid := COALESCE(NEW.voucher_id, OLD.voucher_id);
    v_posted  timestamptz;
BEGIN
    SELECT posted_at INTO v_posted FROM vouchers WHERE id = v_id;

    IF v_posted IS NOT NULL THEN
        RAISE EXCEPTION 'voucher % is posted; its lines are immutable', v_id
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;

    RETURN COALESCE(NEW, OLD);
END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

-- A voucher must balance at COMMIT, not at insert: the header is written before
-- its lines. Deferred so the whole document is one transaction.
CREATE FUNCTION voucher_assert_balanced(p_voucher uuid) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_debit  bigint;
    v_credit bigint;
    v_lines  integer;
BEGIN
    SELECT COALESCE(SUM(amount) FILTER (WHERE side = 'DEBIT'), 0),
           COALESCE(SUM(amount) FILTER (WHERE side = 'CREDIT'), 0),
           COUNT(*)
      INTO v_debit, v_credit, v_lines
      FROM voucher_lines
     WHERE voucher_id = p_voucher;

    IF v_lines = 0 THEN
        RAISE EXCEPTION 'voucher % has no lines', p_voucher
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;

    IF v_debit <> v_credit THEN
        RAISE EXCEPTION 'voucher % is unbalanced: debit %, credit %', p_voucher, v_debit, v_credit
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION vouchers_balanced_trigger() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM voucher_assert_balanced(NEW.id);
    RETURN NEW;
END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

CREATE FUNCTION voucher_lines_balanced_trigger() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM voucher_assert_balanced(COALESCE(NEW.voucher_id, OLD.voucher_id));
    RETURN COALESCE(NEW, OLD);
END
$$;

-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER vouchers_balanced
    AFTER INSERT OR UPDATE OF voucher_no ON vouchers
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION vouchers_balanced_trigger();

CREATE CONSTRAINT TRIGGER voucher_lines_balanced
    AFTER INSERT OR UPDATE OR DELETE ON voucher_lines
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION voucher_lines_balanced_trigger();

CREATE TRIGGER vouchers_immutable
    BEFORE UPDATE OR DELETE ON vouchers
    FOR EACH ROW EXECUTE FUNCTION vouchers_guard();

CREATE TRIGGER voucher_lines_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON voucher_lines
    FOR EACH ROW EXECUTE FUNCTION voucher_lines_guard();

-- +goose StatementBegin

CREATE FUNCTION allocations_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_total      bigint;
    v_allocated  bigint;
BEGIN
    SELECT total INTO v_total FROM documents WHERE id = NEW.document_id;

    SELECT COALESCE(SUM(amount), 0) INTO v_allocated
      FROM allocations
     WHERE document_id = NEW.document_id
       AND id <> COALESCE(NEW.id, '00000000-0000-0000-0000-000000000000'::uuid);

    IF v_allocated + NEW.amount > v_total THEN
        RAISE EXCEPTION 'allocation % exceeds document % (total %, already allocated %)',
            NEW.amount, NEW.document_id, v_total, v_allocated
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;

    RETURN NEW;
END
$$;

-- +goose StatementEnd

CREATE TRIGGER allocations_within_document
    BEFORE INSERT OR UPDATE OF amount ON allocations
    FOR EACH ROW EXECUTE FUNCTION allocations_guard();

-- +goose StatementBegin

CREATE FUNCTION append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only: % refused', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'integrity_constraint_violation';
END
$$;

-- +goose StatementEnd

CREATE TRIGGER audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION append_only();

CREATE TRIGGER period_locks_append_only
    BEFORE UPDATE OR DELETE ON accounting_period_locks
    FOR EACH ROW EXECUTE FUNCTION append_only();

CREATE TRIGGER allocations_append_only
    BEFORE UPDATE OR DELETE ON allocations
    FOR EACH ROW EXECUTE FUNCTION append_only();

CREATE TRIGGER stock_movements_append_only
    BEFORE UPDATE OR DELETE ON stock_movements
    FOR EACH ROW EXECUTE FUNCTION append_only();

-- ---------------------------------------------------------------------------
-- row-level security: no app.client_id => zero rows, never all rows
-- ---------------------------------------------------------------------------

-- +goose StatementBegin

DO $$
DECLARE
    t text;
    scoped text[] := ARRAY[
        'accounts', 'accounting_periods', 'accounting_period_locks',
        'vouchers', 'voucher_lines', 'documents', 'allocations',
        'inventory_policies', 'stock_movements', 'audit_log'
    ];
BEGIN
    FOREACH t IN ARRAY scoped LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format(
            'CREATE POLICY %I ON %I FOR ALL USING (client_id = app_current_client()) WITH CHECK (client_id = app_current_client())',
            t || '_by_client', t);
    END LOOP;
END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

-- Grant the application role its privileges if it exists. Migrations run as the
-- owner; the app role owns nothing, so FORCE RLS applies to it and to the owner.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO fingo_app;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO fingo_app;
    END IF;
END
$$;

-- +goose StatementEnd

-- +goose StatementBegin

-- Provisioning a client copies the statutory chart into that client's accounts.
CREATE FUNCTION provision_client_chart(p_client uuid) RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    v_count integer;
BEGIN
    INSERT INTO accounts (client_id, code, name, parent_code, account_type, nature)
    SELECT p_client, code, name, parent_code, account_type, nature
      FROM chart_template
    ON CONFLICT (client_id, code) DO NOTHING;

    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END
$$;

-- +goose StatementEnd

-- ---------------------------------------------------------------------------
-- seed: the TT133 chart (123 accounts) and one development client
-- ---------------------------------------------------------------------------

INSERT INTO chart_template (code, name, parent_code, account_type, nature) VALUES
    ('111', 'Tiền mặt', NULL, 'ASSET', 'DEBIT'),
    ('1111', 'Tiền Việt Nam', '111', 'ASSET', 'DEBIT'),
    ('1112', 'Ngoại tệ', '111', 'ASSET', 'DEBIT'),
    ('112', 'Tiền gửi Ngân hàng', NULL, 'ASSET', 'DEBIT'),
    ('1121', 'Tiền Việt Nam', '112', 'ASSET', 'DEBIT'),
    ('1122', 'Ngoại tệ', '112', 'ASSET', 'DEBIT'),
    ('121', 'Chứng khoán kinh doanh', NULL, 'ASSET', 'DEBIT'),
    ('128', 'Đầu tư nắm giữ đến ngày đáo hạn', NULL, 'ASSET', 'DEBIT'),
    ('1281', 'Tiền gửi có kỳ hạn', '128', 'ASSET', 'DEBIT'),
    ('1288', 'Các khoản đầu tư khác nắm giữ đến ngày đáo hạn', '128', 'ASSET', 'DEBIT'),
    ('131', 'Phải thu của khách hàng', NULL, 'ASSET', 'DUAL'),
    ('133', 'Thuế GTGT được khấu trừ', NULL, 'ASSET', 'DEBIT'),
    ('1331', 'Thuế GTGT được khấu trừ của hàng hóa, dịch vụ', '133', 'ASSET', 'DEBIT'),
    ('1332', 'Thuế GTGT được khấu trừ của TSCĐ', '133', 'ASSET', 'DEBIT'),
    ('136', 'Phải thu nội bộ', NULL, 'ASSET', 'DEBIT'),
    ('1361', 'Vốn kinh doanh ở đơn vị trực thuộc', '136', 'ASSET', 'DEBIT'),
    ('1368', 'Phải thu nội bộ khác', '136', 'ASSET', 'DEBIT'),
    ('138', 'Phải thu khác', NULL, 'ASSET', 'DEBIT'),
    ('1381', 'Tài sản thiếu chờ xử lý', '138', 'ASSET', 'DEBIT'),
    ('1386', 'Cầm cố, thế chấp, ký quỹ, ký cược', '138', 'ASSET', 'DEBIT'),
    ('1388', 'Phải thu khác', '138', 'ASSET', 'DEBIT'),
    ('141', 'Tạm ứng', NULL, 'ASSET', 'DEBIT'),
    ('151', 'Hàng mua đang đi đường', NULL, 'ASSET', 'DEBIT'),
    ('152', 'Nguyên liệu, vật liệu', NULL, 'ASSET', 'DEBIT'),
    ('153', 'Công cụ, dụng cụ', NULL, 'ASSET', 'DEBIT'),
    ('154', 'Chi phí sản xuất, kinh doanh dở dang', NULL, 'ASSET', 'DEBIT'),
    ('155', 'Thành phẩm', NULL, 'ASSET', 'DEBIT'),
    ('156', 'Hàng hóa', NULL, 'ASSET', 'DEBIT'),
    ('157', 'Hàng gửi đi bán', NULL, 'ASSET', 'DEBIT'),
    ('211', 'Tài sản cố định', NULL, 'ASSET', 'DEBIT'),
    ('2111', 'TSCĐ hữu hình', '211', 'ASSET', 'DEBIT'),
    ('2112', 'TSCĐ thuê tài chính', '211', 'ASSET', 'DEBIT'),
    ('2113', 'TSCĐ vô hình', '211', 'ASSET', 'DEBIT'),
    ('214', 'Hao mòn tài sản cố định', NULL, 'ASSET', 'CREDIT'),
    ('2141', 'Hao mòn TSCĐ hữu hình', '214', 'ASSET', 'DEBIT'),
    ('2142', 'Hao mòn TSCĐ thuê tài chính', '214', 'ASSET', 'DEBIT'),
    ('2143', 'Hao mòn TSCĐ vô hình', '214', 'ASSET', 'DEBIT'),
    ('2147', 'Hao mòn bất động sản đầu tư', '214', 'ASSET', 'DEBIT'),
    ('217', 'Bất động sản đầu tư', NULL, 'ASSET', 'DEBIT'),
    ('228', 'Đầu tư góp vốn vào đơn vị khác', NULL, 'ASSET', 'DEBIT'),
    ('2281', 'Đầu tư vào công ty liên doanh, liên kết', '228', 'ASSET', 'DEBIT'),
    ('2288', 'Đầu tư khác', '228', 'ASSET', 'DEBIT'),
    ('229', 'Dự phòng tổn thất tài sản', NULL, 'ASSET', 'CREDIT'),
    ('2291', 'Dự phòng giảm giá chứng khoán kinh doanh', '229', 'ASSET', 'DEBIT'),
    ('2292', 'Dự phòng tổn thất đầu tư vào đơn vị khác', '229', 'ASSET', 'DEBIT'),
    ('2293', 'Dự phòng phải thu khó đòi', '229', 'ASSET', 'DEBIT'),
    ('2294', 'Dự phòng giảm giá hàng tồn kho', '229', 'ASSET', 'DEBIT'),
    ('241', 'Xây dựng cơ bản dở dang', NULL, 'ASSET', 'DEBIT'),
    ('2411', 'Mua sắm TSCĐ', '241', 'ASSET', 'DEBIT'),
    ('2412', 'Xây dựng cơ bản', '241', 'ASSET', 'DEBIT'),
    ('2413', 'Sửa chữa lớn TSCĐ', '241', 'ASSET', 'DEBIT'),
    ('242', 'Chi phí trả trước', NULL, 'ASSET', 'DEBIT'),
    ('331', 'Phải trả cho người bán', NULL, 'LIABILITY', 'DUAL'),
    ('333', 'Thuế và các khoản phải nộp Nhà nước', NULL, 'LIABILITY', 'CREDIT'),
    ('3331', 'Thuế giá trị gia tăng phải nộp', '333', 'LIABILITY', 'CREDIT'),
    ('33311', 'Thuế GTGT đầu ra', '3331', 'LIABILITY', 'CREDIT'),
    ('33312', 'Thuế GTGT hàng nhập khẩu', '3331', 'LIABILITY', 'CREDIT'),
    ('3332', 'Thuế tiêu thụ đặc biệt', '333', 'LIABILITY', 'CREDIT'),
    ('3333', 'Thuế xuất, nhập khẩu', '333', 'LIABILITY', 'CREDIT'),
    ('3334', 'Thuế thu nhập doanh nghiệp', '333', 'LIABILITY', 'CREDIT'),
    ('3335', 'Thuế thu nhập cá nhân', '333', 'LIABILITY', 'CREDIT'),
    ('3336', 'Thuế tài nguyên', '333', 'LIABILITY', 'CREDIT'),
    ('3337', 'Thuế nhà đất, tiền thuê đất', '333', 'LIABILITY', 'CREDIT'),
    ('3338', 'Thuế bảo vệ môi trường và các loại thuế khác', '333', 'LIABILITY', 'CREDIT'),
    ('33381', 'Thuế bảo vệ môi trường', '3338', 'LIABILITY', 'CREDIT'),
    ('33382', 'Các loại thuế khác', '3338', 'LIABILITY', 'CREDIT'),
    ('3339', 'Phí, lệ phí và các khoản phải nộp khác', '333', 'LIABILITY', 'CREDIT'),
    ('334', 'Phải trả người lao động', NULL, 'LIABILITY', 'CREDIT'),
    ('335', 'Chi phí phải trả', NULL, 'LIABILITY', 'CREDIT'),
    ('336', 'Phải trả nội bộ', NULL, 'LIABILITY', 'CREDIT'),
    ('3361', 'Phải trả nội bộ về vốn kinh doanh', '336', 'LIABILITY', 'CREDIT'),
    ('3368', 'Phải trả nội bộ khác', '336', 'LIABILITY', 'CREDIT'),
    ('338', 'Phải trả, phải nộp khác', NULL, 'LIABILITY', 'CREDIT'),
    ('3381', 'Tài sản thừa chờ giải quyết', '338', 'LIABILITY', 'CREDIT'),
    ('3382', 'Kinh phí công đoàn', '338', 'LIABILITY', 'CREDIT'),
    ('3383', 'Bảo hiểm xã hội', '338', 'LIABILITY', 'CREDIT'),
    ('3384', 'Bảo hiểm y tế', '338', 'LIABILITY', 'CREDIT'),
    ('3385', 'Bảo hiểm thất nghiệp', '338', 'LIABILITY', 'CREDIT'),
    ('3386', 'Nhận ký quỹ, ký cược', '338', 'LIABILITY', 'CREDIT'),
    ('3387', 'Doanh thu chưa thực hiện', '338', 'LIABILITY', 'CREDIT'),
    ('3388', 'Phải trả, phải nộp khác', '338', 'LIABILITY', 'CREDIT'),
    ('341', 'Vay và nợ thuê tài chính', NULL, 'LIABILITY', 'CREDIT'),
    ('3411', 'Các khoản đi vay', '341', 'LIABILITY', 'CREDIT'),
    ('3412', 'Nợ thuê tài chính', '341', 'LIABILITY', 'CREDIT'),
    ('352', 'Dự phòng phải trả', NULL, 'LIABILITY', 'CREDIT'),
    ('3521', 'Dự phòng bảo hành sản phẩm hàng hóa', '352', 'LIABILITY', 'CREDIT'),
    ('3522', 'Dự phòng bảo hành công trình xây dựng', '352', 'LIABILITY', 'CREDIT'),
    ('3524', 'Dự phòng phải trả khác', '352', 'LIABILITY', 'CREDIT'),
    ('353', 'Quỹ khen thưởng phúc lợi', NULL, 'LIABILITY', 'CREDIT'),
    ('3531', 'Quỹ khen thưởng', '353', 'LIABILITY', 'CREDIT'),
    ('3532', 'Quỹ phúc lợi', '353', 'LIABILITY', 'CREDIT'),
    ('3533', 'Quỹ phúc lợi đã hình thành TSCĐ', '353', 'LIABILITY', 'CREDIT'),
    ('3534', 'Quỹ thưởng ban quản lý điều hành công ty', '353', 'LIABILITY', 'CREDIT'),
    ('356', 'Quỹ phát triển khoa học và công nghệ', NULL, 'LIABILITY', 'CREDIT'),
    ('3561', 'Quỹ phát triển khoa học và công nghệ', '356', 'LIABILITY', 'CREDIT'),
    ('3562', 'Quỹ phát triển khoa học và công nghệ đã hình thành TSCĐ', '356', 'LIABILITY', 'CREDIT'),
    ('411', 'Vốn đầu tư của chủ sở hữu', NULL, 'EQUITY', 'CREDIT'),
    ('4111', 'Vốn góp của chủ sở hữu', '411', 'EQUITY', 'CREDIT'),
    ('4112', 'Thặng dư vốn cổ phần', '411', 'EQUITY', 'CREDIT'),
    ('4118', 'Vốn khác', '411', 'EQUITY', 'CREDIT'),
    ('413', 'Chênh lệch tỷ giá hối đoái', NULL, 'EQUITY', 'DUAL'),
    ('418', 'Các quỹ thuộc vốn chủ sở hữu', NULL, 'EQUITY', 'CREDIT'),
    ('419', 'Cổ phiếu quỹ', NULL, 'EQUITY', 'DEBIT'),
    ('421', 'Lợi nhuận sau thuế chưa phân phối', NULL, 'EQUITY', 'DUAL'),
    ('4211', 'Lợi nhuận sau thuế chưa phân phối năm trước', '421', 'EQUITY', 'CREDIT'),
    ('4212', 'Lợi nhuận sau thuế chưa phân phối năm nay', '421', 'EQUITY', 'CREDIT'),
    ('511', 'Doanh thu bán hàng và cung cấp dịch vụ', NULL, 'REVENUE', 'CREDIT'),
    ('5111', 'Doanh thu bán hàng hóa', '511', 'REVENUE', 'CREDIT'),
    ('5112', 'Doanh thu bán thành phẩm', '511', 'REVENUE', 'CREDIT'),
    ('5113', 'Doanh thu cung cấp dịch vụ', '511', 'REVENUE', 'CREDIT'),
    ('5118', 'Doanh thu khác', '511', 'REVENUE', 'CREDIT'),
    ('515', 'Doanh thu hoạt động tài chính', NULL, 'REVENUE', 'CREDIT'),
    ('611', 'Mua hàng', NULL, 'EXPENSE', 'DEBIT'),
    ('631', 'Giá thành sản xuất', NULL, 'EXPENSE', 'DEBIT'),
    ('632', 'Giá vốn hàng bán', NULL, 'EXPENSE', 'DEBIT'),
    ('635', 'Chi phí tài chính', NULL, 'EXPENSE', 'DEBIT'),
    ('642', 'Chi phí quản lý kinh doanh', NULL, 'EXPENSE', 'DEBIT'),
    ('6421', 'Chi phí bán hàng', '642', 'EXPENSE', 'DEBIT'),
    ('6422', 'Chi phí quản lý doanh nghiệp', '642', 'EXPENSE', 'DEBIT'),
    ('711', 'Thu nhập khác', NULL, 'REVENUE', 'CREDIT'),
    ('811', 'Chi phí khác', NULL, 'EXPENSE', 'DEBIT'),
    ('821', 'Chi phí thuế thu nhập doanh nghiệp', NULL, 'EXPENSE', 'DEBIT'),
    ('911', 'Xác định kết quả kinh doanh', NULL, 'EXPENSE', 'DUAL')

ON CONFLICT (code) DO NOTHING;

-- +goose StatementBegin

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fingo_app') THEN
        GRANT EXECUTE ON FUNCTION provision_client_chart(uuid) TO fingo_app;
    END IF;
END
$$;

-- +goose StatementEnd

-- A development client, so a fresh install has books to open. Real deployments
-- create their own; the id is fixed so scripts and tests can name it.
INSERT INTO clients (id, name, tax_id)
VALUES ('00000000-0000-0000-0000-000000000001', 'Công ty TNHH Bình Minh', '0101234567')
ON CONFLICT (tax_id) DO NOTHING;

-- The seed runs as the table owner, and FORCE RLS applies to the owner too, so
-- the client scope must be set for the accounts this migration writes.
SELECT set_config('app.client_id', '00000000-0000-0000-0000-000000000001', true);
SELECT provision_client_chart('00000000-0000-0000-0000-000000000001');

-- +goose Down

DROP FUNCTION IF EXISTS provision_client_chart(uuid);
DROP FUNCTION IF EXISTS append_only() CASCADE;
DROP FUNCTION IF EXISTS allocations_guard() CASCADE;
DROP FUNCTION IF EXISTS voucher_lines_balanced_trigger() CASCADE;
DROP FUNCTION IF EXISTS vouchers_balanced_trigger() CASCADE;
DROP FUNCTION IF EXISTS voucher_assert_balanced(uuid) CASCADE;
DROP FUNCTION IF EXISTS voucher_lines_guard() CASCADE;
DROP FUNCTION IF EXISTS vouchers_guard() CASCADE;
DROP FUNCTION IF EXISTS app_require_client() CASCADE;
DROP FUNCTION IF EXISTS app_current_client() CASCADE;

DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS inventory_policies;
DROP TABLE IF EXISTS allocations;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS voucher_lines;
DROP TABLE IF EXISTS vouchers;
DROP TABLE IF EXISTS accounting_period_locks;
DROP TABLE IF EXISTS accounting_periods;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS chart_template;
DROP TABLE IF EXISTS clients;
