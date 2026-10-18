package migrate_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"fingo/internal/adapter/mariadb/conn"
	"fingo/internal/migrate"
)

// scratchPrefix guards the DROP: this test may only drop databases it named
// itself. The underscore in a LIKE pattern is a wildcard, so the prefix is
// checked in Go and never matched in SQL.
const scratchPrefix = "fingo_migrate_test_"

// config reads the nearest .env by walking up from the test's directory.
func config(t *testing.T) conn.Config {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			cfg, _, err := conn.FromEnvFile(path)
			if err != nil {
				t.Fatalf("config from %s: %v", path, err)
			}

			return cfg
		}

		dir = filepath.Dir(dir)
	}

	t.Skip("no .env found; skipping database integration test")

	return conn.Config{}
}

// scratch creates an empty database and drops it when the test ends.
func scratch(t *testing.T) *sql.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("integration test skipped in short mode")
	}

	cfg := config(t)

	admin, err := sql.Open("mysql", cfg.DSN(""))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { admin.Close() })

	if err := admin.Ping(); err != nil {
		t.Skipf("MariaDB not reachable at %s:%s: %v", cfg.Host, cfg.Port, err)
	}

	name := fmt.Sprintf("%s%d", scratchPrefix, time.Now().UnixNano())
	if !strings.HasPrefix(name, scratchPrefix) {
		t.Fatalf("refusing to create database %q: outside the scratch prefix", name)
	}

	if _, err := admin.Exec(
		"CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		t.Fatalf("create scratch database %s: %v", name, err)
	}

	t.Cleanup(func() {
		if !strings.HasPrefix(name, scratchPrefix) {
			t.Errorf("refusing to drop %q: outside the scratch prefix", name)

			return
		}
		if _, err := admin.Exec("DROP DATABASE IF EXISTS `" + name + "`"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
	})

	db, err := sql.Open("mysql", cfg.DSN(name))
	if err != nil {
		t.Fatalf("open scratch connection: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

// seedLedger puts rows in the database before any migration runs, so that
// "was the data still there afterwards?" has an answer.
func seedLedger(t *testing.T, db *sql.DB) {
	t.Helper()

	mustExec(t, db, "CREATE TABLE ledger (id INT PRIMARY KEY, memo VARCHAR(64) NOT NULL)")
	mustExec(t, db,
		"INSERT INTO ledger (id, memo) VALUES (1, 'một'), (2, 'hai'), (3, 'ba')")
}

// theSet is the migration set the successful-path tests run: one table created,
// one column added to the table that is already there.
func theSet(t *testing.T) []migrate.Migration {
	t.Helper()

	return load(t, migrationFS(map[string]string{
		"0001_notes.sql":       "CREATE TABLE notes (id INT PRIMARY KEY, body VARCHAR(255) NOT NULL);",
		"0002_ledger_flag.sql": "ALTER TABLE ledger ADD COLUMN posted TINYINT NOT NULL DEFAULT 0;",
	}))
}

// The first requirement, and the ordering it insists on: a backup exists, it was
// finished before anything was applied, and it holds the database as it stood.
func TestMigrate_BacksUpAndVerifiesBeforeItRunsAnything(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	root := t.TempDir()

	result, err := migrate.New(db, theSet(t), migrate.WithBackupRoot(root)).Migrate(ctx)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if len(result.Applied) != 2 {
		t.Fatalf("applied %d migrations, want 2", len(result.Applied))
	}
	if result.BackupDir == "" {
		t.Fatal("the migration ran without a backup")
	}

	manifest, err := migrate.ReadManifest(result.BackupDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	// The directory is the one the requirement names: backups/<timestamp>.
	if filepath.Dir(result.BackupDir) != root {
		t.Errorf("backup is in %s, want directly under %s", result.BackupDir, root)
	}
	if _, err := time.Parse("2006-01-02_15-04-05", filepath.Base(result.BackupDir)); err != nil {
		t.Errorf("backup directory %q is not named YYYY-MM-DD_HH-MM-SS", filepath.Base(result.BackupDir))
	}

	// The backup was finished before the first migration was recorded. This is
	// the whole of the guarantee: a backup taken afterwards would be a copy of
	// the damage.
	records := appliedRecords(t, db)
	if len(records) != 2 {
		t.Fatalf("schema_migrations holds %d rows, want 2", len(records))
	}
	if manifest.FinishedAt.After(records[0].AppliedAt) {
		t.Errorf("the backup finished at %s, after migration 1 was applied at %s",
			manifest.FinishedAt, records[0].AppliedAt)
	}
	if manifest.StartedAt.After(manifest.FinishedAt) {
		t.Errorf("the backup started at %s and finished at %s", manifest.StartedAt, manifest.FinishedAt)
	}
	if !reflect.DeepEqual([]int64{records[0].Version, records[1].Version}, []int64{1, 2}) {
		t.Errorf("recorded versions = %d, %d; want 1, 2", records[0].Version, records[1].Version)
	}

	// And it holds what was there, both the rows and the schema.
	data := readFile(t, filepath.Join(result.BackupDir, "data.sql"))
	if !strings.Contains(data, "một") {
		t.Error("data.sql does not hold the ledger rows")
	}
	ddl := readFile(t, filepath.Join(result.BackupDir, "schema.sql"))
	if !strings.Contains(ddl, "CREATE TABLE `ledger`") {
		t.Error("schema.sql does not hold the definition of ledger")
	}

	// The integrity check the requirement asks for: the row counts written into
	// the backup are the row counts the database had.
	if got := recordFor(t, manifest, "ledger"); got != 3 {
		t.Errorf("the backup records %d ledger rows, want 3", got)
	}
}

// A migration that fails halfway leaves a schema that is neither the old one nor
// the new one. The backup is what makes that recoverable.
func TestMigrate_RestoresTheDatabaseWhenAMigrationFails(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	root := t.TempDir()

	broken := load(t, migrationFS(map[string]string{
		"0001_notes.sql": "CREATE TABLE notes (id INT PRIMARY KEY);",
		// Two statements that succeed and change the database, then one that
		// cannot run at all.
		"0002_broken.sql": "INSERT INTO ledger (id, memo) VALUES (4, 'bốn');\n" +
			"DROP TABLE notes;\n" +
			"THIS IS NOT SQL;",
	}))

	result, err := migrate.New(db, broken, migrate.WithBackupRoot(root)).Migrate(ctx)
	if err == nil {
		t.Fatal("a migration that is not SQL was accepted")
	}

	if result.BackupDir == "" {
		t.Fatal("the failure did not name a backup")
	}
	if !strings.Contains(err.Error(), result.BackupDir) {
		t.Errorf("the error does not name the backup it restored from: %v", err)
	}
	if strings.Contains(err.Error(), "could NOT be restored") {
		t.Errorf("the restore failed: %v", err)
	}

	// The row the failed migration inserted is gone.
	if got := scalar(t, db, "SELECT COUNT(*) FROM ledger"); got != 3 {
		t.Errorf("ledger holds %d rows after the restore, want the original 3", got)
	}
	if got := scalar(t, db, "SELECT COUNT(*) FROM ledger WHERE id = 4"); got != 0 {
		t.Error("the row the failed migration inserted is still there")
	}

	// The table the same batch had already created is gone too: the backup
	// precedes the batch, so a failure anywhere restores all of it.
	if tableExists(t, db, "notes") {
		t.Error("notes survived the restore; the batch was not undone as a whole")
	}

	// And the schema came back with the rows, not only the rows: the column
	// migration 0002 would have added to ledger is not there.
	if columnExists(t, db, "ledger", "posted") {
		t.Error("the column the failed batch added is still there")
	}
	if !columnExists(t, db, "ledger", "memo") {
		t.Error("ledger lost a column it had before the migration")
	}

	// And nothing was recorded, so the next run starts from where this one did.
	if got := scalar(t, db, "SELECT COUNT(*) FROM schema_migrations"); got != 0 {
		t.Errorf("schema_migrations holds %d rows after a rolled-back run, want 0", got)
	}
}

// Re-running a migration that is already applied changes nothing, and does not
// take a backup: no migration runs, so the rule has nothing to apply to.
func TestMigrate_DoesNothingWhenEverythingIsAlreadyApplied(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	root := t.TempDir()
	set := theSet(t)

	if _, err := migrate.New(db, set, migrate.WithBackupRoot(root)).Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	before := appliedRecords(t, db)
	backups := backupDirs(t, root)

	second, err := migrate.New(db, set, migrate.WithBackupRoot(root)).Migrate(ctx)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}

	if len(second.Applied) != 0 {
		t.Errorf("re-running applied %d migrations, want 0", len(second.Applied))
	}
	if len(second.Skipped) != 2 {
		t.Errorf("re-running skipped %d, want both", len(second.Skipped))
	}
	if second.BackupDir != "" {
		t.Errorf("re-running took a backup at %s with nothing to do", second.BackupDir)
	}
	if second.Version != 2 {
		t.Errorf("version = %d, want 2", second.Version)
	}

	if after := appliedRecords(t, db); !reflect.DeepEqual(before, after) {
		t.Errorf("the bookkeeping changed on a no-op run:\n before %+v\n after  %+v", before, after)
	}
	if after := backupDirs(t, root); len(after) != len(backups) {
		t.Errorf("a second backup was written: %v then %v", backups, after)
	}
}

// A successful migration adds a table and a column. It must not move a row.
func TestMigrate_LeavesTheRowCountsWhereTheyWere(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	mustExec(t, db, "CREATE TABLE tags (id INT PRIMARY KEY, label VARCHAR(32) NOT NULL)")
	mustExec(t, db, "INSERT INTO tags (id, label) VALUES (1, 'a'), (2, 'b')")

	before := map[string]int64{
		"ledger": scalar(t, db, "SELECT COUNT(*) FROM ledger"),
		"tags":   scalar(t, db, "SELECT COUNT(*) FROM tags"),
	}

	root := t.TempDir()

	result, err := migrate.New(db, theSet(t), migrate.WithBackupRoot(root)).Migrate(ctx)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	for table, want := range before {
		if got := scalar(t, db, "SELECT COUNT(*) FROM "+table); got != want {
			t.Errorf("%s holds %d rows after the migration, want %d", table, got, want)
		}
	}

	// The counts the backup recorded agree with what the migration left behind,
	// which is what makes the backup a restore point for this schema.
	manifest, err := migrate.ReadManifest(result.BackupDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	for table, want := range before {
		if got := recordFor(t, manifest, table); got != want {
			t.Errorf("the backup records %d rows for %s, want %d", got, table, want)
		}
	}

	// The new table is there, and its column is usable.
	mustExec(t, db, "INSERT INTO notes (id, body) VALUES (1, 'ghi chú')")
	mustExec(t, db, "UPDATE ledger SET posted = 1 WHERE id = 1")
}

// An applied migration whose file has changed is not re-run and not skipped. The
// database holds a schema neither the file nor the row describes.
func TestMigrate_RefusesAnAppliedMigrationThatHasBeenEdited(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	root := t.TempDir()

	if _, err := migrate.New(db, theSet(t), migrate.WithBackupRoot(root)).Migrate(ctx); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	edited := load(t, migrationFS(map[string]string{
		"0001_notes.sql":       "CREATE TABLE notes (id INT PRIMARY KEY, body VARCHAR(255) NOT NULL, extra INT NULL);",
		"0002_ledger_flag.sql": "ALTER TABLE ledger ADD COLUMN posted TINYINT NOT NULL DEFAULT 0;",
	}))

	if _, err := migrate.New(db, edited, migrate.WithBackupRoot(root)).Migrate(ctx); err == nil {
		t.Fatal("an edited migration was accepted")
	} else if !strings.Contains(err.Error(), "has been edited") {
		t.Errorf("error = %v, want the drift refusal", err)
	}
}

// A backup whose data file no longer matches its manifest is not a backup, and
// the check that says so has to happen before the migration, not during a
// restore.
func TestVerifyBackup_CatchesATamperedDataFile(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	dir, err := migrate.Backup(ctx, db, t.TempDir(), time.Now)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if err := migrate.VerifyBackup(ctx, db, dir, nil); err != nil {
		t.Fatalf("an untouched backup did not verify: %v", err)
	}

	path := filepath.Join(dir, "data.sql")
	if err := os.WriteFile(path, []byte("-- truncated\n"), 0o600); err != nil {
		t.Fatalf("tamper with the backup: %v", err)
	}

	if err := migrate.VerifyBackup(ctx, db, dir, nil); err == nil {
		t.Fatal("a tampered backup verified")
	}
}

// The row-count half of the check: a backup of a database that has since
// changed is not a restore point for the database as it is now, and the check
// that says so must bite before a migration relies on it.
func TestVerifyBackup_NoticesTheDatabaseHasMovedOn(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	dir, err := migrate.Backup(ctx, db, t.TempDir(), time.Now)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if err := migrate.VerifyBackup(ctx, db, dir, nil); err != nil {
		t.Fatalf("a fresh backup did not verify: %v", err)
	}

	mustExec(t, db, "INSERT INTO ledger (id, memo) VALUES (4, 'bốn')")

	err = migrate.VerifyBackup(ctx, db, dir, nil)
	if err == nil {
		t.Fatal("a backup of a database that has since gained a row verified")
	}
	if !strings.Contains(err.Error(), "ledger") {
		t.Errorf("error = %v, want it to name the table that moved", err)
	}

	// Narrowing the check to the tables that must match is what keyTables is
	// for: a table known to be moving can be left out of it.
	if err := migrate.VerifyBackup(ctx, db, dir, []string{"schema_migrations"}); err != nil {
		t.Errorf("VerifyBackup keyed on an unchanged table: %v", err)
	}
}

// A restore is destructive, so it refuses to run anywhere but the database the
// backup came from.
func TestRestore_RefusesToRunAgainstAnotherDatabase(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	dir, err := migrate.Backup(ctx, db, t.TempDir(), time.Now)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Rewrite the manifest to claim the backup came from somewhere else.
	manifest, err := migrate.ReadManifest(dir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	manifest.Database = "fingo_somewhere_else"

	raw := fmt.Sprintf(`{"database":%q,"server_version":%q,"started_at":%q,"finished_at":%q,"tables":[],"data_checksum":%q}`,
		manifest.Database, manifest.ServerVersion,
		manifest.StartedAt.Format(time.RFC3339Nano), manifest.FinishedAt.Format(time.RFC3339Nano),
		manifest.DataChecksum)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(raw), 0o600); err != nil {
		t.Fatalf("write the manifest: %v", err)
	}

	if err := migrate.Restore(ctx, db, dir); err == nil {
		t.Fatal("a restore ran against the wrong database")
	}

	// Refusing means refusing: the data is untouched.
	if got := scalar(t, db, "SELECT COUNT(*) FROM ledger"); got != 3 {
		t.Errorf("ledger holds %d rows after a refused restore, want 3", got)
	}
}

// FOREIGN_KEY_CHECKS is session state, and the connection a restore used goes
// back into the pool afterwards. Left off, it would silently disable constraint
// checking for whatever borrowed that connection next.
//
// The pool is held to one connection on purpose. That is both the case where a
// leaked flag is certain to be seen, and the case where asking the pool for a
// second connection in the middle of a restore would never return at all.
func TestRestore_LeavesThePoolWithForeignKeyChecksOn(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	dir, err := migrate.Backup(ctx, db, t.TempDir(), time.Now)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if err := migrate.Restore(ctx, db, dir); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	var on int
	if err := db.QueryRow("SELECT @@FOREIGN_KEY_CHECKS").Scan(&on); err != nil {
		t.Fatalf("read FOREIGN_KEY_CHECKS: %v", err)
	}
	if on != 1 {
		t.Error("the connection went back to the pool with foreign key checks off")
	}

	// The flag being on is not the point; the constraint being enforced is.
	mustExec(t, db, "CREATE TABLE parent (id INT PRIMARY KEY)")
	mustExec(t, db, `CREATE TABLE child (
		id INT PRIMARY KEY,
		parent_id INT NULL,
		CONSTRAINT fk_child_parent FOREIGN KEY (parent_id) REFERENCES parent(id))`)

	if _, err := db.Exec("INSERT INTO child (id, parent_id) VALUES (1, 99)"); err == nil {
		t.Error("a foreign key was not enforced after a restore")
	}

	mustExec(t, db, "INSERT INTO parent (id) VALUES (99)")
	mustExec(t, db, "INSERT INTO child (id, parent_id) VALUES (1, 99)")
}

// A corrupt backup must not be restored over a database that is merely part-way
// through a migration. That would replace a schema a person can still fix with
// one nobody can.
func TestRestore_RefusesACorruptBackupBeforeItDropsAnything(t *testing.T) {
	ctx := context.Background()
	db := scratch(t)
	seedLedger(t, db)

	dir, err := migrate.Backup(ctx, db, t.TempDir(), time.Now)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "data.sql"), []byte("-- gone\n"), 0o600); err != nil {
		t.Fatalf("corrupt the backup: %v", err)
	}

	err = migrate.Restore(ctx, db, dir)
	if err == nil {
		t.Fatal("a corrupt backup was restored")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error = %v, want the artifact refusal", err)
	}

	// Refusing means nothing was dropped: the table and its rows are as they
	// were before the restore was attempted.
	if !tableExists(t, db, "ledger") {
		t.Fatal("ledger was dropped by a restore that had refused the backup")
	}
	if got := scalar(t, db, "SELECT COUNT(*) FROM ledger"); got != 3 {
		t.Errorf("ledger holds %d rows, want the original 3", got)
	}
}

// appliedRecords reads the bookkeeping table directly.
func appliedRecords(t *testing.T, db *sql.DB) []migrate.Record {
	t.Helper()

	rows, err := db.Query(
		`SELECT version, name, checksum, applied_at, duration_ms FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	defer rows.Close()

	var out []migrate.Record

	for rows.Next() {
		var r migrate.Record
		if err := rows.Scan(&r.Version, &r.Name, &r.Checksum, &r.AppliedAt, &r.DurationMS); err != nil {
			t.Fatalf("scan schema_migrations: %v", err)
		}

		out = append(out, r)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}

	return out
}

func recordFor(t *testing.T, manifest migrate.Manifest, table string) int64 {
	t.Helper()

	for _, record := range manifest.Tables {
		if record.Name == table {
			return record.Rows
		}
	}

	t.Fatalf("the backup manifest says nothing about %s", table)

	return -1
}

func backupDirs(t *testing.T, root string) []string {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	var out []string

	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, entry.Name())
		}
	}

	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(raw)
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()

	if _, err := db.Exec(query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func scalar(t *testing.T, db *sql.DB, query string) int64 {
	t.Helper()

	var out int64
	if err := db.QueryRow(query).Scan(&out); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}

	return out
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var n int

	err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n)
	if err != nil {
		t.Fatalf("look for table %s: %v", table, err)
	}

	return n > 0
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()

	var n int

	err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
		table, column).Scan(&n)
	if err != nil {
		t.Fatalf("look for %s.%s: %v", table, column, err)
	}

	return n > 0
}
