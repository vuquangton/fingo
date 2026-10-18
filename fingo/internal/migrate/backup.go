package migrate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultBackupRoot is where backups go unless a runner says otherwise. The
// directory named in the requirement: backups/YYYY-MM-DD_HH-MM-SS/.
const DefaultBackupRoot = "backups"

// The files one backup directory holds.
const (
	backupSchemaFile   = "schema.sql"
	backupDataFile     = "data.sql"
	backupManifestFile = "manifest.json"
)

// backupDirLayout names a backup directory. UTC, like every other instant in
// this system: two machines in different zones taking a backup at the same
// moment must not disagree about when it was.
const backupDirLayout = "2006-01-02_15-04-05"

// Errors a backup or a restore can produce.
var (
	// ErrBackupIncomplete is a backup that does not describe the database it
	// claims to. It is the one error that must stop a migration: running the
	// change with no way back is the thing the backup exists to prevent.
	ErrBackupIncomplete = errors.New("migrate: the backup does not match the database")

	// ErrNoBackup is a directory that is not a backup.
	ErrNoBackup = errors.New("migrate: no backup in this directory")

	// ErrWrongDatabase is a restore that would run against a database other
	// than the one the backup was taken from. Refused: a restore is destructive
	// by design, and "which database am I pointed at?" is not a question to be
	// optimistic about.
	ErrWrongDatabase = errors.New("migrate: the backup belongs to a different database")
)

// TableRecord is one table's place in a backup.
type TableRecord struct {
	// Name is the table.
	Name string `json:"name"`

	// Rows is how many rows were written into the backup. This is the number
	// the restore is checked against: a backup that carries fewer rows than the
	// table had is not a backup.
	Rows int64 `json:"rows"`

	// Checksum covers the table's rows as they were written, so two backups can
	// be compared without restoring either. It is not what Verify checks — that
	// is the whole-file checksum below — but it is what a human reads when
	// asking whether anything actually changed.
	Checksum string `json:"checksum"`
}

// Manifest describes one backup.
type Manifest struct {
	// Database is where the backup came from, and the only database a restore
	// from it may be run against.
	Database string `json:"database"`

	// ServerVersion is recorded because a dump restored onto a different major
	// version is a question somebody will ask later.
	ServerVersion string `json:"server_version"`

	// StartedAt and FinishedAt bracket the copy. A migration's own timestamp
	// must fall after FinishedAt: the backup is only a way back if it was
	// finished before the change began.
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`

	// Tables is every table in the database, in dump order.
	Tables []TableRecord `json:"tables"`

	// DataChecksum is the SHA-256 of data.sql. Verification recomputes it from
	// the file: a backup that has been edited, truncated or half-written is not
	// a backup, and this is how that is noticed before it is needed.
	DataChecksum string `json:"data_checksum"`
}

// Backup writes the database to a new directory under root and returns its path.
//
// The dump is logical — the schema from SHOW CREATE TABLE, the rows as INSERT
// statements — which is what mysqldump produces and what can be read by eye when
// something has gone wrong. mysqldump itself is not used: it is a client binary
// a Windows desktop install is not guaranteed to have, it cannot be tested
// without one, and the driver this application already depends on can read every
// byte of the database itself.
//
// The copy is streamed rather than buffered, so a client database larger than
// the machine's memory still has a backup. now is injected so that a caller
// keeping the same clock as the migration record keeps the same clock here.
//
// It reads through a Querier rather than a *sql.DB so that a caller may pin one
// connection for the whole dump: the schema and the rows then come from one
// session, and a pool of one cannot deadlock against itself.
func Backup(ctx context.Context, db Querier, root string, now func() time.Time) (string, error) {
	database, err := currentDatabase(ctx, db)
	if err != nil {
		return "", err
	}

	started := now().UTC()

	dir, err := createBackupDir(root, started)
	if err != nil {
		return "", err
	}

	// From here on the directory exists. A failure leaves it behind, clearly
	// unfinished, rather than deleting the evidence of what went wrong.
	manifest := Manifest{
		Database:      database,
		ServerVersion: serverVersion(ctx, db),
		StartedAt:     started,
	}

	tables, err := listTables(ctx, db)
	if err != nil {
		return dir, err
	}

	if err := writeSchema(ctx, db, filepath.Join(dir, backupSchemaFile), tables); err != nil {
		return dir, err
	}

	records, err := writeData(ctx, db, filepath.Join(dir, backupDataFile), tables)
	if err != nil {
		return dir, err
	}
	manifest.Tables = records

	sum, err := hashFile(filepath.Join(dir, backupDataFile))
	if err != nil {
		return dir, err
	}
	manifest.DataChecksum = sum
	manifest.FinishedAt = now().UTC()

	if err := writeManifest(dir, manifest); err != nil {
		return dir, err
	}

	return dir, nil
}

// VerifyBackup checks that a backup still describes the database it came from.
//
// Two things are checked, and they answer different questions.
//
// The file checksum answers "is this artifact intact?" — it is recomputed from
// data.sql and compared with the manifest, so a truncated or edited dump is
// caught now rather than during a restore.
//
// The row counts answer "does this artifact match the database?" — every table's
// live count is compared with the count written into the backup. A row count
// that moved between the dump and the check means the database changed while it
// was being copied, and a backup of a moving target is not a restore point.
//
// keyTables narrows the second check to the tables that matter. Empty means
// every table, which is what a migration uses: a schema change does not announce
// in advance which table it will touch.
//
// It reads through the Querier it is given, so a caller already holding a
// connection verifies on that connection instead of waiting for a second one.
func VerifyBackup(ctx context.Context, db Querier, dir string, keyTables []string) error {
	manifest, err := verifyArtifact(dir)
	if err != nil {
		return err
	}

	return verifyCounts(ctx, db, manifest, keyTables)
}

// verifyArtifact checks the backup directory itself: that the manifest reads and
// that data.sql is still the file the manifest describes.
//
// This half needs no database, which is the point of it being separate. A
// restore runs it before it drops anything: restoring from a corrupt backup over
// a merely broken schema would turn a recoverable failure into an unrecoverable
// one.
func verifyArtifact(dir string) (Manifest, error) {
	manifest, err := ReadManifest(dir)
	if err != nil {
		return Manifest{}, err
	}

	got, err := hashFile(filepath.Join(dir, backupDataFile))
	if err != nil {
		return Manifest{}, err
	}
	if got != manifest.DataChecksum {
		return Manifest{}, fmt.Errorf("%w: %s hashes to %s, the manifest says %s",
			ErrBackupIncomplete, backupDataFile, got, manifest.DataChecksum)
	}

	return manifest, nil
}

// verifyCounts checks the database against what the backup recorded.
func verifyCounts(ctx context.Context, db Querier, manifest Manifest, keyTables []string) error {
	live, err := tableRowCounts(ctx, db)
	if err != nil {
		return err
	}

	key := keyset(keyTables)

	for _, table := range manifest.Tables {
		if len(key) > 0 && !key[table.Name] {
			continue
		}

		count, ok := live[table.Name]
		if !ok {
			return fmt.Errorf("%w: the backup holds %s and the database does not",
				ErrBackupIncomplete, table.Name)
		}
		if count != table.Rows {
			return fmt.Errorf("%w: %s has %d rows now and the backup holds %d",
				ErrBackupIncomplete, table.Name, count, table.Rows)
		}
	}

	return nil
}

// ReadManifest reads a backup directory's manifest.
func ReadManifest(dir string) (Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, backupManifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, fmt.Errorf("%w: %s", ErrNoBackup, dir)
		}

		return Manifest{}, fmt.Errorf("read the backup manifest in %s: %w", dir, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: %s holds an unreadable manifest", ErrNoBackup, dir)
	}
	if manifest.Database == "" {
		return Manifest{}, fmt.Errorf("%w: %s names no database", ErrNoBackup, dir)
	}

	return manifest, nil
}

// writeManifest writes the manifest, indented so it can be read and diffed.
func writeManifest(dir string, manifest Manifest) error {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the backup manifest: %w", err)
	}

	path := filepath.Join(dir, backupManifestFile)
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// createBackupDir makes the backup directory for an instant.
//
// The name is the requirement's: YYYY-MM-DD_HH-MM-SS. Two backups inside one
// second would collide, which a test does routinely and a migration does when
// two clients are upgraded in the same tick, so a suffix is added rather than
// the first backup being overwritten.
func createBackupDir(root string, at time.Time) (string, error) {
	base := filepath.Join(root, at.UTC().Format(backupDirLayout))

	for i := 0; i < 100; i++ {
		dir := base
		if i > 0 {
			dir = base + "-" + strconv.Itoa(i)
		}

		entries, err := os.ReadDir(dir)
		switch {
		case err == nil:
			// An existing directory is used only when it is empty. A backup
			// directory is never written over.
			if len(entries) == 0 {
				return dir, nil
			}
		case os.IsNotExist(err):
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return "", fmt.Errorf("create backup directory %s: %w", dir, err)
			}

			return dir, nil
		default:
			return "", fmt.Errorf("read backup directory %s: %w", dir, err)
		}
	}

	return "", fmt.Errorf("create backup directory under %s: too many backups inside one second", root)
}

// writeSchema writes every table's CREATE statement.
//
// FOREIGN_KEY_CHECKS is switched off in the file itself, not only by the
// restorer. The statements are written in name order, which is not a dependency
// order, and a file that only loads when the reader remembers to disable the
// checks is a file somebody will one day load without doing so.
func writeSchema(ctx context.Context, db Querier, path string, tables []string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	header := "-- Schema of " + filepath.Base(filepath.Dir(path)) +
		", written by internal/migrate.\nSET FOREIGN_KEY_CHECKS=0;\n\n"
	if _, err := w.WriteString(header); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	for _, table := range tables {
		var (
			name string
			ddl  string
		)

		if err := db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+table+"`").Scan(&name, &ddl); err != nil {
			return fmt.Errorf("read the definition of %s: %w", table, err)
		}

		if _, err := fmt.Fprintf(w, "-- %s\n%s;\n\n", table, ddl); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	if _, err := w.WriteString("SET FOREIGN_KEY_CHECKS=1;\n"); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// writeData writes every table's rows as INSERT statements and returns a record
// of what went in.
func writeData(ctx context.Context, db Querier, path string, tables []string) ([]TableRecord, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	out := bufio.NewWriter(f)

	if _, err := out.WriteString("SET FOREIGN_KEY_CHECKS=0;\n"); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}

	records := make([]TableRecord, 0, len(tables))

	for _, table := range tables {
		record, err := copyTable(ctx, db, out, table)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	if _, err := out.WriteString("SET FOREIGN_KEY_CHECKS=1;\n"); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}

	if err := out.Flush(); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}

	return records, nil
}

// copyTable writes one table's rows, hashing them as they go.
func copyTable(ctx context.Context, db Querier, out io.Writer, table string) (TableRecord, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM `"+table+"`")
	if err != nil {
		return TableRecord{}, fmt.Errorf("read %s: %w", table, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return TableRecord{}, fmt.Errorf("read the columns of %s: %w", table, err)
	}

	types, err := rows.ColumnTypes()
	if err != nil {
		return TableRecord{}, fmt.Errorf("read the column types of %s: %w", table, err)
	}

	typeNames := make([]string, len(types))
	for i, t := range types {
		typeNames[i] = strings.ToUpper(t.DatabaseTypeName())
	}

	tableHash := sha256.New()
	section := io.MultiWriter(out, tableHash)

	if _, err := fmt.Fprintf(section, "\n-- %s\n", table); err != nil {
		return TableRecord{}, fmt.Errorf("write %s: %w", table, err)
	}

	prefix := "INSERT INTO `" + table + "` (`" + strings.Join(columns, "`, `") + "`) VALUES ("

	var count int64

	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return TableRecord{}, fmt.Errorf("scan a row of %s: %w", table, err)
		}

		literals := make([]string, len(values))
		for i, v := range values {
			literals[i] = literal(v, typeNames[i])
		}

		if _, err := fmt.Fprintf(section, "%s%s);\n", prefix, strings.Join(literals, ", ")); err != nil {
			return TableRecord{}, fmt.Errorf("write a row of %s: %w", table, err)
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return TableRecord{}, fmt.Errorf("iterate %s: %w", table, err)
	}

	return TableRecord{
		Name:     table,
		Rows:     count,
		Checksum: hex.EncodeToString(tableHash.Sum(nil)),
	}, nil
}

// literal renders one value as a SQL literal.
//
// Everything textual is quoted, including numbers: '3' is a valid INT, and this
// way no value's type has to be guessed from the column it came from. Dates are
// the one case that needs the declared type — a DATE written as a DATETIME is a
// truncation MySQL reports as a warning nobody reads — so the caller passes it.
func literal(v any, typeName string) string {
	switch value := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return quote(value)
	case string:
		return quote([]byte(value))
	case bool:
		if value {
			return "1"
		}

		return "0"
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64)
	case time.Time:
		switch typeName {
		case "DATE":
			return quote([]byte(value.Format("2006-01-02")))
		case "TIME":
			return quote([]byte(value.Format("15:04:05.999999")))
		default:
			return quote([]byte(value.Format("2006-01-02 15:04:05.999999")))
		}
	default:
		return quote([]byte(fmt.Sprint(value)))
	}
}

// quote wraps bytes in single quotes, escaping what MySQL reads specially.
//
// Backslash escaping rather than only doubling the quote: a backslash in the
// data would otherwise escape whatever follows it, and a dump that renders
// correctly for the common case and corrupts a Windows path is worse than no
// dump at all.
//
// Known limit: a column declared BINARY, VARBINARY or BLOB would be better
// written as a hex literal (0x...), because bytes above 0x7f are interpreted
// through the connection's character set rather than kept as bytes. Nothing in
// this schema has one. This is where that changes when something does.
func quote(raw []byte) string {
	var b strings.Builder

	b.Grow(len(raw) + 2)
	b.WriteByte('\'')

	for _, c := range raw {
		switch c {
		case '\'':
			b.WriteString(`\'`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case 0:
			b.WriteString(`\0`)
		case 0x1a:
			b.WriteString(`\Z`)
		default:
			b.WriteByte(c)
		}
	}

	b.WriteByte('\'')

	return b.String()
}

// listTables returns the database's own tables, in a fixed order.
//
// information_schema, not SHOW TABLES: the query can be ordered and filtered in
// the server, and views are excluded because a view is a question rather than a
// fact — recreating one is a separate concern from restoring the data under it.
func listTables(ctx context.Context, db Querier) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("list the database's tables: %w", err)
	}
	defer rows.Close()

	var out []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan a table name: %w", err)
		}

		out = append(out, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate the database's tables: %w", err)
	}

	return out, nil
}

// tableRowCounts returns every table's row count.
func tableRowCounts(ctx context.Context, db Querier) (map[string]int64, error) {
	tables, err := listTables(ctx, db)
	if err != nil {
		return nil, err
	}

	out := make(map[string]int64, len(tables))

	for _, table := range tables {
		var count int64

		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+table+"`").Scan(&count); err != nil {
			return nil, fmt.Errorf("count the rows of %s: %w", table, err)
		}

		out[table] = count
	}

	return out, nil
}

// currentDatabase returns the database the connection is pointed at.
func currentDatabase(ctx context.Context, db Querier) (string, error) {
	var name sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", fmt.Errorf("read the current database: %w", err)
	}
	if !name.Valid || name.String == "" {
		return "", errors.New("migrate: the connection names no database")
	}

	return name.String, nil
}

// serverVersion returns the server's version, or "" if it cannot be read.
//
// Worth recording, not worth failing a backup over.
func serverVersion(ctx context.Context, db Querier) string {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return ""
	}

	return version
}

// hashFile returns the SHA-256 of a file, read in chunks.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}

	return hex.EncodeToString(sum.Sum(nil)), nil
}

// keyset turns a list of names into a set, for the key-table filter.
func keyset(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}

	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[strings.TrimSpace(name)] = true
	}

	return out
}
