// Package singleton keeps one copy of FinGo running at a time.
//
// Two copies would start two database servers against one data directory.
// MariaDB already refuses the second, so the books are safe either way — but the
// second copy fails with a raw server error, which sends the operator to read a
// log file instead of telling them the application is already open.
//
// The guard is a named kernel object rather than a lock file, and that choice is
// the whole design. A lock file left behind by a crash keeps the application
// locked out until somebody finds and deletes it; a kernel object is released by
// the operating system the moment the process ends, however it ends.
package singleton

import (
	"errors"
	"fmt"
	"strings"
)

// ErrAlreadyRunning reports that another copy holds the name.
var ErrAlreadyRunning = errors.New("another copy of this application is already running")

// Guard is this process's claim on the application's name.
type Guard struct {
	name   string
	handle uintptr
}

// Acquire claims the name, or reports ErrAlreadyRunning.
func Acquire(name string) (*Guard, error) {
	name = strings.TrimSpace(name)

	switch {
	case name == "":
		// A guard with no name guards nothing: every caller would claim the same
		// nameless object, and the first copy would lock every other installation
		// on the machine out.
		return nil, errors.New("a single-instance guard needs a name")

	case strings.ContainsAny(name, `\/`):
		// A kernel object name is path-like, so a separator here is not a
		// character. Letting it through fails later at CreateMutex with "the
		// system cannot find the path specified", which says nothing about the
		// real problem. Use NameFor to build a usable name from a path.
		return nil, fmt.Errorf(
			"the guard name %q holds a path separator, which a kernel object name cannot: use NameFor", name)
	}

	handle, err := claim(name)
	if err != nil {
		return nil, err
	}

	return &Guard{name: name, handle: handle}, nil
}

// NameFor derives a guard name from an installation root.
//
// It exists so that no caller has to know what a kernel object name may contain.
// An installation root is a Windows path and a kernel object name is not, so
// passing the path straight through is the obvious mistake — and one that fails
// with an error about paths.
//
// The result is stable: a second launch derives the same name, which is the
// whole of what makes the guard work.
func NameFor(root string) string {
	var b strings.Builder

	b.WriteString("FinGo-")

	for _, r := range root {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	return b.String()
}

// Name is what this guard is guarding.
func (g *Guard) Name() string { return g.name }

// Release gives the claim up.
//
// Safe to call more than once: shutdown runs on every exit path and some of them
// run twice. A guard already released has no handle to close, so the second call
// does nothing rather than closing whatever the handle now means.
func (g *Guard) Release() error {
	if g == nil || g.handle == 0 {
		return nil
	}

	handle := g.handle
	g.handle = 0

	release(handle)

	return nil
}
