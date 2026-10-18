//go:build !windows

package singleton

import (
	"errors"
	"runtime"
)

// claim is unimplemented off Windows.
//
// FinGo is a Windows desktop application, so this cannot be reached by the
// product. It exists so the package still compiles on another platform and says
// why, rather than failing with a build error that explains nothing — the same
// reason the repository keeps its database tests skippable rather than absent.
func claim(string) (uintptr, error) {
	return 0, errors.New("the single-instance guard is only implemented on Windows, not " + runtime.GOOS)
}

func release(uintptr) {}
