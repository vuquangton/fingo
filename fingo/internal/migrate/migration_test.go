package migrate_test

import (
	"errors"
	"testing"
	"testing/fstest"

	"fingo/internal/migrate"
)

// migrationFS turns a set of files into a filesystem the loader can read.
func migrationFS(files map[string]string) fstest.MapFS {
	out := make(fstest.MapFS, len(files))

	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}

	return out
}

func TestLoad_ReadsNumberedFilesInOrder(t *testing.T) {
	set, err := migrate.Load(migrationFS(map[string]string{
		"0002_add_note.sql": "ALTER TABLE ledger ADD COLUMN note VARCHAR(64) NULL;",
		"0001_ledger.sql":   "CREATE TABLE ledger (id INT PRIMARY KEY);",
	}), ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(set) != 2 {
		t.Fatalf("loaded %d migrations, want 2", len(set))
	}
	if set[0].Version != 1 || set[1].Version != 2 {
		t.Fatalf("versions = %d, %d; want 1, 2", set[0].Version, set[1].Version)
	}
	if set[1].Name != "add_note" {
		t.Errorf("name = %q, want %q", set[1].Name, "add_note")
	}
	if len(set[0].Statements) != 1 {
		t.Errorf("statements = %d, want 1", len(set[0].Statements))
	}
}

// A file that carries goose's markers is read for its Up half. That is what lets
// this runner read a directory this repository already has.
func TestLoad_TakesTheUpHalfOfAGooseMarkedFile(t *testing.T) {
	set, err := migrate.Load(migrationFS(map[string]string{
		"0001_notes.sql": `-- +goose Up
CREATE TABLE notes (id INT PRIMARY KEY);

-- +goose Down
DROP TABLE notes;
`,
	}), ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(set) != 1 {
		t.Fatalf("loaded %d migrations, want 1", len(set))
	}
	if len(set[0].Statements) != 1 {
		t.Fatalf("statements = %q, want just the CREATE", set[0].Statements)
	}
	if set[0].Statements[0] != "CREATE TABLE notes (id INT PRIMARY KEY)" {
		t.Errorf("statement = %q", set[0].Statements[0])
	}
}

func TestLoad_RefusesASetThatIsNotSequential(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  error
	}{
		"a gap": {
			files: map[string]string{
				"0001_a.sql": "SELECT 1;",
				"0003_c.sql": "SELECT 3;",
			},
			want: migrate.ErrVersionGap,
		},
		"nothing starts at 1": {
			files: map[string]string{
				"0002_b.sql": "SELECT 2;",
				"0003_c.sql": "SELECT 3;",
			},
			want: migrate.ErrVersionGap,
		},
		"a duplicate": {
			files: map[string]string{
				"0001_a.sql":  "SELECT 1;",
				"0001_aa.sql": "SELECT 11;",
			},
			want: migrate.ErrDuplicateVersion,
		},
		"a malformed name": {
			files: map[string]string{
				"ledger.sql": "SELECT 1;",
			},
			want: migrate.ErrBadFilename,
		},
		"an empty up section": {
			files: map[string]string{
				"0001_a.sql": "-- nothing here\n",
			},
			want: migrate.ErrEmptyMigration,
		},
		"no files at all": {
			files: map[string]string{},
			want:  migrate.ErrNoMigrations,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := migrate.Load(migrationFS(tc.files), ".")
			if !errors.Is(err, tc.want) {
				t.Fatalf("Load = %v, want %v", err, tc.want)
			}
		})
	}
}

// The checksum covers the whole file, Down section included, so that any edit to
// an applied migration is drift and not just the executable half of one.
func TestLoad_ChecksumFollowsTheWholeFile(t *testing.T) {
	base := migrationFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 2;\n",
	})
	editedUp := migrationFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 11;\n\n-- +goose Down\nSELECT 2;\n",
	})
	editedDown := migrationFS(map[string]string{
		"0001_a.sql": "-- +goose Up\nSELECT 1;\n\n-- +goose Down\nSELECT 22;\n",
	})

	first := load(t, base)
	if got := load(t, editedUp); got[0].Checksum == first[0].Checksum {
		t.Error("editing the Up section did not change the checksum")
	}
	if got := load(t, editedDown); got[0].Checksum == first[0].Checksum {
		t.Error("editing the Down section did not change the checksum")
	}
}

func load(t *testing.T, fsys fstest.MapFS) []migrate.Migration {
	t.Helper()

	set, err := migrate.Load(fsys, ".")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return set
}
