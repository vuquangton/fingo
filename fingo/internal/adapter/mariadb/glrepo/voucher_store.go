// Package glrepo stores and retrieves general-ledger vouchers in MariaDB.
package glrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	mariadb "fingo/internal/adapter/mariadb/sqlc"
	"fingo/internal/domain/gl"
)

// mysqlDuplicateEntry is MySQL's ER_DUP_ENTRY. MariaDB uses the same number.
const mysqlDuplicateEntry = 1062

// VoucherStore is the MariaDB adapter for gl.VoucherRepository.
type VoucherStore struct {
	db      *sql.DB
	queries *mariadb.Queries
}

var _ gl.VoucherRepository = (*VoucherStore)(nil)

func NewVoucherStore(db *sql.DB) *VoucherStore {
	return &VoucherStore{db: db, queries: mariadb.New(db)}
}

// SaveVoucher writes the voucher and every one of its lines in a single
// transaction of its own.
//
// Atomicity is the whole point: a failure part-way through would leave a header
// with no lines, or a partial line set — an unbalanced voucher sitting in the
// ledger that nothing could repair.
//
// An idempotencyKey names the request this voucher answers. It is recorded in
// the same transaction, so the request and its voucher either both exist or
// neither does — a resubmitted request finds the key already taken and stores
// nothing.
//
// Use this when the voucher is the whole of the write. When a posting also moves
// stock or records an invoice, those must share its transaction — see
// postingrepo.Save.
func (s *VoucherStore) SaveVoucher(ctx context.Context, v *gl.Voucher, idempotencyKey string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save voucher %s: begin: %w", v.ID(), err)
	}
	// No-op once the transaction has committed.
	defer func() { _ = tx.Rollback() }()

	if err := SaveWithin(ctx, s.queries.WithTx(tx), v, idempotencyKey); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save voucher %s: commit: %w", v.ID(), err)
	}

	return nil
}

// SaveWithin writes a voucher using queries that are already inside a
// transaction.
//
// The same write SaveVoucher performs; what differs is who owns the transaction.
// That is what lets a posting record the voucher and everything the document it
// records entails as one unit — see postingrepo.
//
// Storage keys lines by accounts.id for referential integrity, so account codes
// are resolved here. The domain only ever speaks codes.
func SaveWithin(ctx context.Context, q *mariadb.Queries, v *gl.Voucher, idempotencyKey string) error {
	// Stored as a draft first, its lines added, and only then marked posted.
	//
	// That is not ceremony: the immutability trigger refuses to add a line to a
	// voucher that is already posted, so writing the header as posted would make
	// its own lines impossible. The lifecycle the trigger enforces is the
	// lifecycle the domain has, and the transaction keeps the intermediate draft
	// invisible to anyone else.
	if err := q.CreateVoucher(ctx, mariadb.CreateVoucherParams{
		ID:          v.ID(),
		Book:        sql.NullString{},
		FiscalYear:  sql.NullInt16{},
		Seq:         sql.NullInt32{},
		VoucherNo:   "",
		VoucherDate: v.VoucherDate(),
		PostedDate:  sql.NullTime{},
		PostedAt:    sql.NullTime{},
		PostedBy:    sql.NullString{},
		CreatedBy:   sql.NullString{},
		VoucherType: mariadb.VouchersVoucherType(v.VoucherType()),
		Description: nullString(v.Description()),
		IsPosted:    false,
	}); err != nil {
		return fmt.Errorf("save voucher %s: header: %w", v.ID(), err)
	}

	resolved := make(map[string]string)

	for i, l := range v.Lines() {
		accountID, err := resolveAccount(ctx, q, resolved, l.AccountCode)
		if err != nil {
			return fmt.Errorf("save voucher %s: %w", v.ID(), err)
		}

		lineID := l.ID
		if lineID == "" {
			lineID = uuid.NewString()
		}

		order := int32(l.LineOrder)
		if order == 0 {
			order = int32(i + 1)
		}

		if err := q.CreateVoucherLine(ctx, mariadb.CreateVoucherLineParams{
			ID:        lineID,
			VoucherID: v.ID(),
			LineOrder: order,
			AccountID: accountID,
			PartnerID: sql.NullString{String: l.PartnerID, Valid: l.PartnerID != ""},
			Side:      mariadb.VoucherLinesSide(l.Side),
			Amount:    l.Amount.String(),
			Note:      nullString(l.Note),
		}); err != nil {
			return fmt.Errorf("save voucher %s: line %s: %w", v.ID(), l.AccountCode, err)
		}
	}

	if v.IsPosted() {
		book, fiscalYear, seq, err := numberColumns(v)
		if err != nil {
			return err
		}

		if err := q.MarkVoucherPosted(ctx, mariadb.MarkVoucherPostedParams{
			Book:       book,
			FiscalYear: fiscalYear,
			Seq:        seq,
			VoucherNo:  v.VoucherNo(),
			PostedDate: postedDate(v),
			PostedAt:   nullTime(v.PostedAt()),
			PostedBy:   nullString(v.PostedBy()),
			ID:         v.ID(),
		}); err != nil {
			return fmt.Errorf("save voucher %s: mark posted: %w", v.ID(), err)
		}
	}

	if idempotencyKey != "" {
		if err := q.CreateIdempotencyKey(ctx, mariadb.CreateIdempotencyKeyParams{
			KeyValue:  idempotencyKey,
			VoucherID: v.ID(),
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			return duplicateOr(ctx, q, idempotencyKey, err)
		}
	}

	return nil
}

// duplicateOr turns a key collision into the domain's sentinel.
//
// The unique index is the real guard: two transactions racing on the same key
// cannot both succeed, whatever the pre-check saw. A duplicate key error is
// therefore not an internal failure but the answer to the question, and the
// caller gets ErrDuplicateRequest for it.
func duplicateOr(ctx context.Context, queries *mariadb.Queries, key string, err error) error {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != mysqlDuplicateEntry {
		return fmt.Errorf("record request %s: %w", key, err)
	}

	// The transaction rolls back, so the voucher this attempt wrote is gone and
	// only the original remains.
	original, lookupErr := queries.FindVoucherByIdempotencyKey(ctx, key)
	if lookupErr != nil {
		return fmt.Errorf("%w (key %s)", gl.ErrDuplicateRequest, key)
	}

	return fmt.Errorf("%w (key %s, voucher %s)", gl.ErrDuplicateRequest, key, original)
}

// GetVoucher reads a voucher back and rehydrates it through the domain, so
// stored state is re-validated rather than trusted.
func (s *VoucherStore) GetVoucher(ctx context.Context, id string) (*gl.Voucher, error) {
	row, err := s.queries.GetVoucherByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get voucher %s: header: %w", id, err)
	}

	lineRows, err := s.queries.ListVoucherLinesByVoucherID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get voucher %s: lines: %w", id, err)
	}

	lines := make([]gl.VoucherLine, 0, len(lineRows))
	for _, lr := range lineRows {
		amount, err := decimal.NewFromString(lr.Amount)
		if err != nil {
			return nil, fmt.Errorf("get voucher %s: line %s: unreadable amount %q: %w",
				id, lr.ID, lr.Amount, err)
		}

		lines = append(lines, gl.VoucherLine{
			ID:          lr.ID,
			VoucherID:   lr.VoucherID,
			LineOrder:   int(lr.LineOrder),
			AccountCode: lr.AccountCode,
			PartnerID:   lr.PartnerID.String,
			Side:        gl.EntrySide(lr.Side),
			Amount:      amount,
			Note:        lr.Note.String,
		})
	}

	v, err := gl.Rehydrate(gl.PersistedVoucher{
		ID:          row.ID,
		Book:        gl.Book(row.Book.String),
		FiscalYear:  int(row.FiscalYear.Int16),
		Seq:         int(row.Seq.Int32),
		VoucherDate: row.VoucherDate,
		VoucherType: gl.VoucherType(row.VoucherType),
		Description: row.Description.String,
		IsPosted:    row.IsPosted,
		PostedAt:    row.PostedAt.Time,
		PostedBy:    row.PostedBy.String,
		Lines:       lines,
	})
	if err != nil {
		return nil, fmt.Errorf("get voucher %s: %w", id, err)
	}

	return v, nil
}

// NextNumber allocates the next place in a book's run for a fiscal year.
//
// Allocation runs in its own transaction. That is a deliberate trade: the
// alternative is for this adapter to reach back and number a domain voucher,
// which would put the ledger's numbering inside its storage. The cost is that a
// crash between allocating and saving leaves a gap — and a gap in a book is
// something a bookkeeper expects to see and explain, not a silent corruption.
func (s *VoucherStore) NextNumber(ctx context.Context, book gl.Book, fiscalYear int) (gl.Number, error) {
	if !book.Valid() {
		return gl.Number{}, fmt.Errorf("number book %q is not one of the known series", book)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return gl.Number{}, fmt.Errorf("allocate %s %d: begin: %w", book, fiscalYear, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	if err := q.EnsureVoucherSequence(ctx, mariadb.EnsureVoucherSequenceParams{
		Book:       string(book),
		FiscalYear: int16(fiscalYear),
	}); err != nil {
		return gl.Number{}, fmt.Errorf("allocate %s %d: %w", book, fiscalYear, err)
	}

	next, err := q.ReserveVoucherSequence(ctx, mariadb.ReserveVoucherSequenceParams{
		Book:       string(book),
		FiscalYear: int16(fiscalYear),
	})
	if err != nil {
		return gl.Number{}, fmt.Errorf("allocate %s %d: reserve: %w", book, fiscalYear, err)
	}

	if err := q.AdvanceVoucherSequence(ctx, mariadb.AdvanceVoucherSequenceParams{
		NextSeq:    next + 1,
		Book:       string(book),
		FiscalYear: int16(fiscalYear),
	}); err != nil {
		return gl.Number{}, fmt.Errorf("allocate %s %d: advance: %w", book, fiscalYear, err)
	}

	if err := tx.Commit(); err != nil {
		return gl.Number{}, fmt.Errorf("allocate %s %d: commit: %w", book, fiscalYear, err)
	}

	return gl.Number{Book: book, Year: fiscalYear, Seq: int(next)}, nil
}

// numberColumns returns the stored number columns for a voucher.
//
// A draft holds no place in a run, so all three come back NULL rather than
// (” , 0, 0) — which would make every draft collide on the unique key.
func numberColumns(v *gl.Voucher) (sql.NullString, sql.NullInt16, sql.NullInt32, error) {
	if !v.IsPosted() {
		return sql.NullString{}, sql.NullInt16{}, sql.NullInt32{}, nil
	}

	number := v.Number()
	if err := number.Validate(); err != nil {
		return sql.NullString{}, sql.NullInt16{}, sql.NullInt32{},
			fmt.Errorf("save voucher %s: %w", v.ID(), err)
	}

	return sql.NullString{String: string(number.Book), Valid: true},
		sql.NullInt16{Int16: int16(number.Year), Valid: true},
		sql.NullInt32{Int32: int32(number.Seq), Valid: true},
		nil
}

// resolveAccount maps an account code to its storage identity, caching within a
// single save so a voucher touching the same account twice reads it once.
func resolveAccount(ctx context.Context, q *mariadb.Queries, cache map[string]string, code string) (string, error) {
	if id, ok := cache[code]; ok {
		return id, nil
	}

	account, err := q.GetAccountByCode(ctx, code)
	if err != nil {
		return "", fmt.Errorf("account %s is not in this client's chart: %w", code, err)
	}

	cache[code] = account.ID

	return account.ID, nil
}

// postedDate is the business date of posting. It is present only once the
// voucher is posted, which is what makes the column meaningful.
func postedDate(v *gl.Voucher) sql.NullTime {
	if !v.IsPosted() {
		return sql.NullTime{}
	}
	return nullTime(v.PostedAt())
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}
