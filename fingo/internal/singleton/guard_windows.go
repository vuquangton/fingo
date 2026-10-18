//go:build windows

package singleton

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

// claim creates the named mutex, or reports ErrAlreadyRunning.
//
// Called through the procedure rather than through windows.CreateMutex because
// the generated wrapper cannot express the answer: CreateMutexW *succeeds* when
// the object already exists — it hands back a second handle to the same object —
// so the only place ERROR_ALREADY_EXISTS appears is the last error.
//
// The name lives in the session namespace, not Global\, so two Windows users on
// one machine each keep their own guard. That is what a per-user installation
// wants: a second user is a second installation, not a second copy.
func claim(name string) (uintptr, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, fmt.Errorf("the guard name %q is not usable: %w", name, err)
	}

	// bInitialOwner is false on purpose. The claim is the open handle, not
	// ownership of the mutex, so Release is a plain CloseHandle with no thread
	// affinity to reason about.
	r1, _, lastErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if r1 == 0 {
		return 0, fmt.Errorf("claim %q: %w", name, lastErr)
	}

	if errors.Is(lastErr, windows.ERROR_ALREADY_EXISTS) {
		_, _, _ = procCloseHandle.Call(r1)

		return 0, ErrAlreadyRunning
	}

	return r1, nil
}

func release(handle uintptr) {
	_, _, _ = procCloseHandle.Call(handle)
}
