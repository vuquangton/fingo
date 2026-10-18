package dbhost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// envBinDir lets a development machine point at a server that has not been
// copied into an installation yet.
const envBinDir = "FINGO_MARIADB_BIN"

// FindBinDir locates the server's binaries for an installation.
//
// Three places, in the order they should be trusted:
//
//	<appDir>/server/bin   the server this copy of FinGo shipped with. First,
//	                      because it is the version the build was tested
//	                      against, and the others are guesses about the machine.
//	<root>/server/bin     a server unpacked into the installation root, which is
//	                      where an installer that cannot write beside the
//	                      executable — the ordinary case under Program Files —
//	                      has to put it.
//	FINGO_MARIADB_BIN     development only, for a machine where nothing has been
//	                      copied into an installation yet.
//
// The shipped server sits beside the executable rather than in the user's data
// directory because it is a read-only program file and the data directory holds
// the books. Mixing them means an uninstall either leaves 36 MB of program
// behind or takes somebody's data with it.
//
// When none of them is there, the error names all three: "server not found"
// sends somebody to read the source.
func FindBinDir(root, appDir string) (string, error) {
	shipped := filepath.Join(appDir, "server", "bin")
	if holds(shipped) {
		return shipped, nil
	}

	unpacked := filepath.Join(root, "server", "bin")
	if holds(unpacked) {
		return unpacked, nil
	}

	override := strings.TrimSpace(os.Getenv(envBinDir))
	if override != "" && holds(override) {
		return override, nil
	}

	where := fmt.Sprintf(
		"no database server found: %s and %s hold no mariadbd.exe",
		shipped, unpacked)

	if override != "" {
		return "", fmt.Errorf("%s, and %s=%s does not either", where, envBinDir, override)
	}

	return "", fmt.Errorf("%s, and %s is not set", where, envBinDir)
}

// holds reports whether a directory has a server in it.
func holds(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "mariadbd.exe"))

	return err == nil && !info.IsDir()
}
