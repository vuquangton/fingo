package dbhost_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fingo/internal/dbhost"
)

// TestNewLayout_KeepsEverythingUnderOneRoot — slice 1a.
//
// A client's installation has to be removable by deleting one directory. If any
// file the server creates lands outside the root, uninstalling leaves something
// behind and nobody knows what it was.
func TestNewLayout_KeepsEverythingUnderOneRoot(t *testing.T) {
	root := `C:\Users\someone\AppData\Local\FinGo`

	l := dbhost.NewLayout(root)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"bin", l.Bin, root + `\server\bin`},
		{"data", l.Data, root + `\server\data`},
		{"log", l.Log, root + `\server\mysqld.log`},
		{"pid", l.PID, root + `\server\mysqld.pid`},
		{"socket", l.Pipe, root + `\server\mysql.sock`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
			if !strings.HasPrefix(tt.got, root) {
				t.Errorf("%s = %q, which is outside the root %q", tt.name, tt.got, root)
			}
		})
	}
}

// TestConfig_ArgsStartAPrivateServer — slice 1a.
//
// Three things matter here and each has a failure mode that is silent:
//
//  1. `--no-defaults` must come first. Without it the bundled server reads the
//     machine's my.ini, and a client who happens to have MySQL installed finds
//     FinGo's server obeying a stranger's configuration.
//  2. It must bind to the loopback address only. A bundled server listening on
//     every interface is a database exposed to the office network.
//  3. The data directory, port and pid file must all be the ones we named, so
//     nothing is shared with any other installation on the machine.
func TestConfig_ArgsStartAPrivateServer(t *testing.T) {
	cfg := dbhost.Config{
		Datadir: `D:\data\fingo\server\data`,
		Port:    33107,
		LogFile: `D:\data\fingo\server\mysqld.log`,
		PIDFile: `D:\data\fingo\server\mysqld.pid`,
	}

	args := cfg.Args()

	if len(args) == 0 {
		t.Fatal("Args() is empty")
	}
	if args[0] != "--no-defaults" {
		t.Errorf("args[0] = %q, want --no-defaults first: a bundled server must not read the machine's my.ini", args[0])
	}

	for _, want := range []string{
		`--datadir=D:\data\fingo\server\data`,
		"--port=33107",
		"--bind-address=127.0.0.1",
		`--log-error=D:\data\fingo\server\mysqld.log`,
		`--pid-file=D:\data\fingo\server\mysqld.pid`,
	} {
		if !contains(args, want) {
			t.Errorf("Args() is missing %q\ngot: %v", want, args)
		}
	}

	// It must not name a user or a defaults file: the bundled server authenticates
	// by being reachable only from this machine, not by a password we would have
	// to store.
	for _, unwanted := range []string{"--defaults-file", "--user="} {
		for _, a := range args {
			if strings.HasPrefix(a, unwanted) {
				t.Errorf("Args() carries %q, which it has no business setting", a)
			}
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}

	return false
}

// TestWaitReady_ReturnsAsSoonAsTheServerAnswers — slice 1b.
//
// Starting a server is not the same as the server being ready: mariadbd accepts
// a connection before it can serve a query. Everything downstream — migrate, the
// chart, the first screen — depends on knowing the difference, so readiness is
// decided by asking the server, not by waiting a fixed time.
func TestWaitReady_ReturnsAsSoonAsTheServerAnswers(t *testing.T) {
	calls := 0

	probe := func(context.Context) error {
		calls++
		return nil
	}

	if err := dbhost.WaitReady(t.Context(), probe, time.Millisecond); err != nil {
		t.Fatalf("WaitReady = %v, want nil when the server answers at once", err)
	}
	if calls != 1 {
		t.Errorf("probe was called %d times, want 1", calls)
	}
}

// TestWaitReady_KeepsAskingWhileTheServerComesUp — slice 1b: a server that is
// not ready yet is not a failure. The first two attempts fail here and the third
// succeeds, which is what a slow start looks like.
func TestWaitReady_KeepsAskingWhileTheServerComesUp(t *testing.T) {
	calls := 0

	probe := func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}

		return nil
	}

	if err := dbhost.WaitReady(t.Context(), probe, time.Millisecond); err != nil {
		t.Fatalf("WaitReady = %v, want nil once the third attempt succeeds", err)
	}
	if calls != 3 {
		t.Errorf("probe was called %d times, want 3", calls)
	}
}

// TestWaitReady_GivesUpWhenTheContextIsDone — slice 1b: waiting forever is how a
// splash screen becomes a hang. The deadline is the caller's, and it is honoured.
func TestWaitReady_GivesUpWhenTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()

	calls := 0

	probe := func(context.Context) error {
		calls++
		return errors.New("connection refused")
	}

	err := dbhost.WaitReady(ctx, probe, time.Millisecond)

	if err == nil {
		t.Fatal("WaitReady = nil, want an error when the server never answers")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitReady = %v, want it to report the deadline", err)
	}
	if calls == 0 {
		t.Error("the probe was never called")
	}
}

// TestWaitReady_StopsAskingOnceTheContextIsDone — slice 1b: a probe that keeps
// running after the caller has given up is a caller that cannot decide anything,
// and the only symptom is a process that will not close.
func TestWaitReady_StopsAskingOnceTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	calls := 0

	probe := func(context.Context) error {
		calls++
		cancel()

		return errors.New("connection refused")
	}

	if err := dbhost.WaitReady(ctx, probe, time.Millisecond); err == nil {
		t.Fatal("WaitReady = nil, want an error after cancellation")
	}

	settled := calls
	time.Sleep(20 * time.Millisecond)

	if calls != settled {
		t.Errorf("the probe kept running after cancellation: %d calls became %d", settled, calls)
	}
}

// TestWaitReadyOrExit_StopsWhenTheServerItselfDies — found by running the
// application, and the most user-visible defect in P0.
//
// A bundled server that cannot open its data directory aborts within a second:
// "InnoDB: The data file './ibdata1' must be writable", then "Aborting". But
// readiness is judged by dialling a port, and a port that nothing ever listens
// on looks exactly like a server that is still starting.
//
// So the operator watched "đang khởi động…" for the full five-minute timeout
// before being told anything. Waiting for a process that has already exited is
// not waiting; it is a hang with a deadline.
func TestWaitReadyOrExit_StopsWhenTheServerItselfDies(t *testing.T) {
	exited := make(chan struct{})

	probe := func(context.Context) error { return errors.New("connection refused") }

	go func() {
		time.Sleep(15 * time.Millisecond)
		close(exited)
	}()

	start := time.Now()
	err := dbhost.WaitReadyOrExit(t.Context(), probe, exited, time.Millisecond)

	if err == nil {
		t.Fatal("WaitReadyOrExit = nil, want an error when the server died")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("it waited %s after the server had already exited", elapsed.Round(time.Millisecond))
	}
}

// TestWaitReadyOrExit_SucceedsWhenTheServerComesUp — the ordinary path is
// unchanged: a server that starts is waited for, and nothing else ends the wait.
func TestWaitReadyOrExit_SucceedsWhenTheServerComesUp(t *testing.T) {
	exited := make(chan struct{})
	calls := 0

	probe := func(context.Context) error {
		calls++
		if calls < 2 {
			return errors.New("connection refused")
		}

		return nil
	}

	if err := dbhost.WaitReadyOrExit(t.Context(), probe, exited, time.Millisecond); err != nil {
		t.Fatalf("WaitReadyOrExit = %v, want nil once the server answers", err)
	}
}

// TestWaitReadyOrExit_HonoursTheDeadlineWhenNothingHappens — neither ready nor
// dead is still a case, and it is still bounded.
func TestWaitReadyOrExit_HonoursTheDeadlineWhenNothingHappens(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()

	exited := make(chan struct{}) // never closed
	probe := func(context.Context) error { return errors.New("connection refused") }

	err := dbhost.WaitReadyOrExit(ctx, probe, exited, time.Millisecond)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitReadyOrExit = %v, want it to report the deadline", err)
	}
}

// TestSupervisor_DoneReportsTheRunningServer — the caller needs the process's
// exit to tell "still starting" from "already dead", and the supervisor is what
// knows which process is running.
func TestSupervisor_DoneReportsTheRunningServer(t *testing.T) {
	launcher := &fakeLauncher{}
	s := dbhost.NewSupervisor(launcher)

	if s.Done() != nil {
		t.Error("Done() is not nil when nothing has been started")
	}

	if err := s.EnsureStarted(t.Context(), dbhost.Config{Port: 33107}); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}

	done := s.Done()
	if done == nil {
		t.Fatal("Done() is nil while a server is running")
	}

	select {
	case <-done:
		t.Error("Done() is already closed while the server is running")
	default:
	}

	if err := s.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-done:
	default:
		t.Error("Done() was not closed after the server stopped")
	}

	if s.Done() != nil {
		t.Error("Done() is not nil after the server stopped")
	}
}

// fakeProcess is a server that never really ran.
type fakeProcess struct {
	done    chan struct{}
	stopped bool
	stopErr error
}

func newFakeProcess() *fakeProcess { return &fakeProcess{done: make(chan struct{})} }

func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Err() error            { return nil }

func (p *fakeProcess) Stop(context.Context) error {
	if p.stopped {
		return p.stopErr
	}

	p.stopped = true
	close(p.done)

	return p.stopErr
}

// fakeLauncher counts how many servers were started, and can refuse to start one.
type fakeLauncher struct {
	starts   int
	proc     *fakeProcess
	startErr error
}

func (l *fakeLauncher) Start(dbhost.Config) (dbhost.Process, error) {
	l.starts++

	if l.startErr != nil {
		return nil, l.startErr
	}
	if l.proc == nil {
		l.proc = newFakeProcess()
	}

	return l.proc, nil
}

// TestSupervisor_EnsureStartedIsIdempotent — slice 1c.
//
// Boot happens on every launch, and the second call must not start a second
// server against the same data directory. Two servers on one datadir is how a
// client's books get corrupted, and the failure is not immediate or obvious.
func TestSupervisor_EnsureStartedIsIdempotent(t *testing.T) {
	launcher := &fakeLauncher{}
	s := dbhost.NewSupervisor(launcher)

	if err := s.EnsureStarted(t.Context(), dbhost.Config{Port: 33107}); err != nil {
		t.Fatalf("first EnsureStarted: %v", err)
	}
	if err := s.EnsureStarted(t.Context(), dbhost.Config{Port: 33107}); err != nil {
		t.Fatalf("second EnsureStarted: %v", err)
	}

	if launcher.starts != 1 {
		t.Errorf("the launcher started %d servers, want 1", launcher.starts)
	}
	if !s.Running() {
		t.Error("Running() is false after a successful start")
	}
}

// TestSupervisor_StopShutsTheServerDown — slice 1c: a server left running after
// the application closes holds the data directory open, and the next launch
// finds it locked.
func TestSupervisor_StopShutsTheServerDown(t *testing.T) {
	launcher := &fakeLauncher{}
	s := dbhost.NewSupervisor(launcher)

	if err := s.EnsureStarted(t.Context(), dbhost.Config{Port: 33107}); err != nil {
		t.Fatalf("EnsureStarted: %v", err)
	}

	if err := s.Stop(t.Context()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if s.Running() {
		t.Error("Running() is still true after Stop")
	}
	if !launcher.proc.stopped {
		t.Error("the process was not asked to stop")
	}
}

// TestSupervisor_StopWithoutAStartIsNotAnError — slice 1c: shutdown runs on every
// exit path, including the ones where the server never started. A shutdown that
// can fail on a machine where nothing went wrong is a shutdown that hides the
// real error.
func TestSupervisor_StopWithoutAStartIsNotAnError(t *testing.T) {
	s := dbhost.NewSupervisor(&fakeLauncher{})

	if err := s.Stop(t.Context()); err != nil {
		t.Errorf("Stop with nothing running = %v, want nil", err)
	}
	if s.Running() {
		t.Error("Running() is true when nothing was started")
	}
}

// TestSupervisor_AFailedStartLeavesNothingRunning — slice 1c: a launcher that
// refused must not leave the supervisor believing a server is up, or every
// later step is attempted against a server that is not there.
func TestSupervisor_AFailedStartLeavesNothingRunning(t *testing.T) {
	launcher := &fakeLauncher{startErr: errors.New("mariadbd.exe not found")}
	s := dbhost.NewSupervisor(launcher)

	err := s.EnsureStarted(t.Context(), dbhost.Config{Port: 33107})

	if err == nil {
		t.Fatal("EnsureStarted = nil, want the launcher's error")
	}
	if !strings.Contains(err.Error(), "mariadbd.exe not found") {
		t.Errorf("EnsureStarted = %v, want the launcher's own words", err)
	}
	if s.Running() {
		t.Error("Running() is true after a failed start")
	}

	// And a later attempt must try again rather than believing it is up.
	if err := s.EnsureStarted(t.Context(), dbhost.Config{Port: 33107}); err == nil {
		t.Error("a second EnsureStarted after a failure reported success")
	}
	if launcher.starts != 2 {
		t.Errorf("the launcher was called %d times, want 2", launcher.starts)
	}
}
