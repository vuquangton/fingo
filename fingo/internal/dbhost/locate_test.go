package dbhost_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fingo/internal/dbhost"
)

// seedServer puts a mariadbd.exe where the caller says, so a layout can be
// tested without shipping a server.
func seedServer(t *testing.T, dir string) string {
	t.Helper()

	bin := filepath.Join(dir, "server", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("create %s: %v", bin, err)
	}
	if err := os.WriteFile(filepath.Join(bin, "mariadbd.exe"), []byte("not really a server"), 0o644); err != nil {
		t.Fatalf("write mariadbd.exe: %v", err)
	}

	return bin
}

// TestFindBinDir_PrefersTheServerShippedWithTheApp — slice 1e.
//
// The server that came with this copy of FinGo wins, and it wins over a server
// in the data root as well as over one further afield. It is the version the
// build was tested against; the others are guesses about this machine.
//
// It sits beside the executable rather than in the user's data directory because
// it is a read-only program file, and the data directory is the user's books.
// Mixing them means an uninstall either leaves 36 MB of program behind or takes
// somebody's data with it.
func TestFindBinDir_PrefersTheServerShippedWithTheApp(t *testing.T) {
	app := t.TempDir()
	shipped := seedServer(t, app)

	root := t.TempDir()
	seedServer(t, root) // a server unpacked into the data root

	t.Setenv("FINGO_MARIADB_BIN", seedServer(t, t.TempDir()))

	got, err := dbhost.FindBinDir(root, app)
	if err != nil {
		t.Fatalf("FindBinDir: %v", err)
	}
	if got != shipped {
		t.Errorf("FindBinDir = %q, want the shipped %q", got, shipped)
	}
}

// TestFindBinDir_FallsBackToTheInstallationRoot — an installer that cannot write
// beside the executable, which is the ordinary case under Program Files, puts
// the server in the data root instead. That still has to work.
func TestFindBinDir_FallsBackToTheInstallationRoot(t *testing.T) {
	app := t.TempDir() // nothing shipped
	root := t.TempDir()
	inRoot := seedServer(t, root)

	t.Setenv("FINGO_MARIADB_BIN", seedServer(t, t.TempDir()))

	got, err := dbhost.FindBinDir(root, app)
	if err != nil {
		t.Fatalf("FindBinDir: %v", err)
	}
	if got != inRoot {
		t.Errorf("FindBinDir = %q, want the data-root server %q", got, inRoot)
	}
}

// TestFindBinDir_FallsBackToTheEnvironment — for development, where nothing has
// been copied into an installation yet.
func TestFindBinDir_FallsBackToTheEnvironment(t *testing.T) {
	app := t.TempDir()
	root := t.TempDir()
	override := seedServer(t, t.TempDir())

	t.Setenv("FINGO_MARIADB_BIN", override)

	got, err := dbhost.FindBinDir(root, app)
	if err != nil {
		t.Fatalf("FindBinDir: %v", err)
	}
	if got != override {
		t.Errorf("FindBinDir = %q, want the override %q", got, override)
	}
}

// TestFindBinDir_SaysWhereItLooked — "server not found" sends somebody to read
// the source. Naming every place makes it one line of investigation, and the
// shipped location is the one worth naming first because it is the one that
// should have been there.
func TestFindBinDir_SaysWhereItLooked(t *testing.T) {
	app := t.TempDir()
	root := t.TempDir()

	t.Setenv("FINGO_MARIADB_BIN", filepath.Join(t.TempDir(), "nothing-here"))

	_, err := dbhost.FindBinDir(root, app)
	if err == nil {
		t.Fatal("FindBinDir succeeded with no server anywhere")
	}

	for _, want := range []string{
		filepath.Join(app, "server", "bin"),
		filepath.Join(root, "server", "bin"),
		"FINGO_MARIADB_BIN",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}
