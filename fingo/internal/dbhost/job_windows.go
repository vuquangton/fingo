//go:build windows

package dbhost

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Job is a Windows job object.
//
// Every process assigned to it is killed when the last handle to the job closes,
// and that handle closes when this process ends — however it ends. It is the
// operating system's answer to "make sure my children do not outlive me", and it
// is the only answer that works for a crash, where a deferred cleanup never runs.
type Job struct {
	handle windows.Handle
}

// NewJob creates a job whose members are killed when it is closed.
func NewJob() (*Job, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create the job object: %w", err)
	}

	job := &Job{handle: handle}

	// KILL_ON_JOB_CLOSE is what makes the job a guarantee rather than a
	// container. Without it the job still exists and still tracks its members,
	// and closing it leaves every one of them running.
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(handle)

		return nil, fmt.Errorf("set KILL_ON_JOB_CLOSE: %w", err)
	}

	return job, nil
}

// Assign puts a running process into the job.
//
// The process handle is opened by id rather than taken from os.Process, which
// does not expose its handle. The handle is closed as soon as the assignment is
// made: the job keeps its own reference, and holding a second one would keep the
// process object alive after it had exited.
func (j *Job) Assign(p *os.Process) error {
	if j == nil || j.handle == 0 {
		return errors.New("the job object is not open")
	}
	if p == nil {
		return errors.New("there is no process to assign")
	}

	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return fmt.Errorf("open process %d: %w", p.Pid, err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	if err := windows.AssignProcessToJobObject(j.handle, handle); err != nil {
		return fmt.Errorf("assign process %d to the job: %w", p.Pid, err)
	}

	return nil
}

// Close destroys the job, killing everything in it. Safe to call more than once.
func (j *Job) Close() error {
	if j == nil || j.handle == 0 {
		return nil
	}

	handle := j.handle
	j.handle = 0

	if err := windows.CloseHandle(handle); err != nil {
		return fmt.Errorf("close the job object: %w", err)
	}

	return nil
}
