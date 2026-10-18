package singleton_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"fingo/internal/singleton"
)

// name builds a mutex name that no other test and no other process is using.
//
// The guard is a real named kernel object, so tests that shared a name would
// fight each other and pass or fail depending on the order they ran in.
func name(t *testing.T) string {
	t.Helper()

	sanitised := strings.NewReplacer("/", "-", " ", "_", "\\", "-").Replace(t.Name())

	return fmt.Sprintf("FinGo-test-%d-%s", os.Getpid(), sanitised)
}

// TestAcquire_ClaimsTheName — the ordinary case: nothing else is running, so the
// application takes its name and starts.
func TestAcquire_ClaimsTheName(t *testing.T) {
	g, err := singleton.Acquire(name(t))
	if err != nil {
		t.Fatalf("Acquire on a free name: %v", err)
	}
	t.Cleanup(func() { _ = g.Release() })

	if g.Name() == "" {
		t.Error("the guard does not know what it is guarding")
	}
}

// TestAcquire_RefusesASecondCopy — the whole point.
//
// Two copies of FinGo would start two servers against one data directory.
// MariaDB already refuses that, so the books are not corrupted — but the second
// copy fails with a raw server error, and the operator is told to look at a log
// file instead of being told the application is already open.
func TestAcquire_RefusesASecondCopy(t *testing.T) {
	n := name(t)

	first, err := singleton.Acquire(n)
	if err != nil {
		t.Fatalf("the first Acquire: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := singleton.Acquire(n)

	if err == nil {
		_ = second.Release()
		t.Fatal("a second Acquire succeeded, so two copies can run at once")
	}
	if !errors.Is(err, singleton.ErrAlreadyRunning) {
		t.Errorf("second Acquire = %v, want ErrAlreadyRunning", err)
	}
}

// TestAcquire_WorksAgainAfterRelease — closing FinGo and reopening it must work.
// A guard that stays claimed after a clean exit makes the application a
// one-shot, and the only fix an operator can find is to reboot.
func TestAcquire_WorksAgainAfterRelease(t *testing.T) {
	n := name(t)

	first, err := singleton.Acquire(n)
	if err != nil {
		t.Fatalf("the first Acquire: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	second, err := singleton.Acquire(n)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	t.Cleanup(func() { _ = second.Release() })
}

// TestAcquire_DifferentNamesAreIndependent — two installations on one machine
// are two installations, not one. Each guards its own data directory.
func TestAcquire_DifferentNamesAreIndependent(t *testing.T) {
	a, err := singleton.Acquire(name(t) + "-a")
	if err != nil {
		t.Fatalf("Acquire a: %v", err)
	}
	t.Cleanup(func() { _ = a.Release() })

	b, err := singleton.Acquire(name(t) + "-b")
	if err != nil {
		t.Fatalf("Acquire b: %v", err)
	}
	t.Cleanup(func() { _ = b.Release() })
}

// TestRelease_IsIdempotent — shutdown runs on every exit path, and some of them
// run twice. Releasing a guard twice must not be an error, and must not release
// somebody else's.
func TestRelease_IsIdempotent(t *testing.T) {
	g, err := singleton.Acquire(name(t))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if err := g.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := g.Release(); err != nil {
		t.Errorf("second Release = %v, want nil", err)
	}
}

// TestAcquire_RefusesAnEmptyName — a guard with no name guards nothing: every
// caller would claim the same nameless object and the first copy would lock
// every other installation on the machine out.
func TestAcquire_RefusesAnEmptyName(t *testing.T) {
	for _, bad := range []string{"", "   "} {
		if g, err := singleton.Acquire(bad); err == nil {
			_ = g.Release()
			t.Errorf("Acquire(%q) succeeded, want a refusal", bad)
		}
	}
}

// TestAcquire_RefusesANameAKernelObjectCannotHave — found by running the
// application, not by a test, and that is the point of writing it down.
//
// A kernel object name is path-like, so a backslash is a separator rather than a
// character. Passing an installation root straight through therefore fails at
// CreateMutex with "the system cannot find the path specified" — an error that
// says nothing about the real problem, from a layer that has no business
// guessing what the caller meant.
func TestAcquire_RefusesANameAKernelObjectCannotHave(t *testing.T) {
	for _, bad := range []string{
		`FinGo-C:\Users\someone\AppData\Local\FinGo`,
		`FinGo/usr/local/fingo`,
	} {
		if g, err := singleton.Acquire(bad); err == nil {
			_ = g.Release()
			t.Errorf("Acquire(%q) succeeded, want a refusal", bad)
		}
	}
}

// TestNameFor_BuildsAUsableNameFromAnInstallationRoot — the helper exists so no
// caller has to know what a kernel object name may contain.
//
// It has to be stable: a second launch derives the name again, and a name that
// differed between launches would guard nothing at all.
func TestNameFor_BuildsAUsableNameFromAnInstallationRoot(t *testing.T) {
	root := `C:\Users\someone\AppData\Local\FinGo`

	derived := singleton.NameFor(root)

	if strings.ContainsAny(derived, `\/`) {
		t.Fatalf("NameFor(%q) = %q, which still holds a path separator", root, derived)
	}
	if derived != singleton.NameFor(root) {
		t.Error("NameFor is not stable, so a second launch would not find the first")
	}

	g, err := singleton.Acquire(derived)
	if err != nil {
		t.Fatalf("Acquire(NameFor(root)): %v", err)
	}
	t.Cleanup(func() { _ = g.Release() })

	// Two installations are two names.
	other := singleton.NameFor(`C:\Users\someone\AppData\Local\FinGo-2`)
	if other == derived {
		t.Error("two installation roots derived the same name")
	}
}
