//go:build !windows

package dbhost

import (
	"errors"
	"os"
	"runtime"
)

// Job is a Windows job object and has no counterpart here.
//
// FinGo ships as a Windows desktop application, so this cannot be reached by the
// product. It exists so the package still compiles elsewhere and says why, in
// the same spirit as the database tests that skip rather than disappear.
type Job struct{}

// NewJob cannot be implemented off Windows.
func NewJob() (*Job, error) {
	return nil, errors.New("job objects are only implemented on Windows, not " + runtime.GOOS)
}

// Assign cannot be implemented off Windows.
func (j *Job) Assign(*os.Process) error {
	return errors.New("job objects are only implemented on Windows, not " + runtime.GOOS)
}

// Close does nothing, since nothing was ever confined.
func (j *Job) Close() error { return nil }
