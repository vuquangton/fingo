package dbhost_test

import (
	"os/exec"
	"testing"
	"time"

	"fingo/internal/dbhost"
)

// child starts a real process that will keep running.
//
// ping rather than a sleep: it is present on every Windows machine, it is not a
// shell (so nothing has to be quoted), and it stays alive for minutes, which is
// long enough to be sure that its death was caused rather than coincidental.
func child(t *testing.T) *exec.Cmd {
	t.Helper()

	cmd := exec.Command("ping", "-n", "300", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a child process on this machine: %v", err)
	}

	return cmd
}

// TestJob_KillsItsChildrenWhenItIsClosed — slice 1f, and the fix for a defect
// found by running the application.
//
// Force-killing FinGo left mariadbd running and holding the data directory. The
// next launch then failed with "InnoDB: The data file './ibdata1' must be
// writable", and the application could not be started again until somebody
// found the process in Task Manager. A clean close stops the server correctly;
// a crash must not need to be clean.
//
// A job object is the operating system's answer: every process assigned to it is
// killed when the last handle to the job closes, and the last handle closes when
// the process holding it ends — however it ends.
func TestJob_KillsItsChildrenWhenItIsClosed(t *testing.T) {
	job, err := dbhost.NewJob()
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	t.Cleanup(func() { _ = job.Close() })

	cmd := child(t)

	if err := job.Assign(cmd.Process); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	// cmd.Wait must be called exactly once, and it is what tells us the child
	// died. Go's contract, not ours.
	waited := make(chan struct{})

	go func() {
		_ = cmd.Wait()
		close(waited)
	}()

	// It is alive to begin with, or the rest proves nothing.
	select {
	case <-waited:
		t.Fatal("the child exited before the job was closed")
	case <-time.After(400 * time.Millisecond):
	}

	if err := job.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-waited:
	case <-time.After(15 * time.Second):
		t.Fatal("the child outlived the job that was supposed to kill it")
	}
}

// TestJob_CloseIsIdempotent — shutdown runs on every exit path, and some run
// twice. Closing an already-closed job must not close whatever the handle now
// means.
func TestJob_CloseIsIdempotent(t *testing.T) {
	job, err := dbhost.NewJob()
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}

	if err := job.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := job.Close(); err != nil {
		t.Errorf("second Close = %v, want nil", err)
	}
}

// TestJob_RefusesToAssignNothing — assigning a process that is not running is a
// mistake in the caller, not something to discover later.
func TestJob_RefusesToAssignNothing(t *testing.T) {
	job, err := dbhost.NewJob()
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	t.Cleanup(func() { _ = job.Close() })

	if err := job.Assign(nil); err == nil {
		t.Error("Assign(nil) succeeded, want a refusal")
	}
}
