// Package dbhost owns a bundled database server's lifetime.
//
// FinGo ships its own MariaDB so a client install needs no server, no
// credentials and no configuration file. This package is the part of that which
// can be reasoned about without a server running: where its files live, how it
// is invoked, and when it is ready.
package dbhost

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
)

// Layout is where a bundled server keeps its files.
//
// Everything lives under one root, so a client's installation is removable by
// deleting one directory. A file the server writes outside the root is a file
// uninstalling leaves behind, and nobody then knows what it was.
type Layout struct {
	Root string
	Bin  string
	Data string
	Log  string
	PID  string
	Pipe string
}

// NewLayout derives the file layout for an installation root.
func NewLayout(root string) Layout {
	server := filepath.Join(root, "server")

	return Layout{
		Root: root,
		Bin:  filepath.Join(server, "bin"),
		Data: filepath.Join(server, "data"),
		Log:  filepath.Join(server, "mysqld.log"),
		PID:  filepath.Join(server, "mysqld.pid"),
		Pipe: filepath.Join(server, "mysql.sock"),
	}
}

// Config is how a bundled server is started.
type Config struct {
	// MysqldExe is the server binary, and AdminExe the client used to ask it to
	// stop. AdminExe is optional: without it the process is killed instead,
	// which works and recovers on the next launch, but it is not the path a
	// client's books should take on every exit.
	MysqldExe string
	AdminExe  string

	Datadir string
	Port    int
	LogFile string
	PIDFile string
}

// Args is the server's command line, without the executable.
//
// Three things here are load-bearing, and each fails silently:
//
//  1. **`--no-defaults` first.** Otherwise the bundled server reads whatever
//     my.ini the machine happens to have, and a client who also runs MySQL finds
//     FinGo's server obeying a stranger's configuration.
//  2. **`--bind-address=127.0.0.1`.** A bundled server listening on every
//     interface is a database exposed to the office network.
//  3. **An explicit port, datadir, log and pid file.** Nothing may be shared
//     with another installation on the same machine.
//
// No `--user` and no defaults file: the server authenticates by being reachable
// only from this machine, not by a password we would have to store somewhere.
func (c Config) Args() []string {
	return []string{
		"--no-defaults",
		"--datadir=" + c.Datadir,
		"--port=" + strconv.Itoa(c.Port),
		"--bind-address=127.0.0.1",
		"--log-error=" + c.LogFile,
		"--pid-file=" + c.PIDFile,
	}
}

// Probe reports whether the server is answering yet.
type Probe func(ctx context.Context) error

// WaitReady polls probe until it answers or the context is done.
//
// Starting a process is not the same as the process being ready: mariadbd
// accepts a TCP connection before it can serve a query, so a caller that dials
// and stops there will migrate against a server that is still opening its
// tables. Readiness is therefore decided by asking, not by sleeping a fixed
// time — a fixed sleep is wrong on a slow machine and wasteful on a fast one.
//
// Waiting forever is how a splash screen becomes a hang, so the deadline is the
// caller's and it is honoured. The probe is never called again once the context
// is done: a probe still running after the caller gave up is a caller that
// cannot decide anything, and the only symptom is a process that will not close.
func WaitReady(ctx context.Context, probe Probe, interval time.Duration) error {
	return WaitReadyOrExit(ctx, probe, nil, interval)
}

// WaitReadyOrExit is WaitReady, and also gives up when the server itself dies.
//
// This is not a refinement. A bundled server that cannot open its data directory
// aborts within a second — "InnoDB: The data file './ibdata1' must be writable",
// then "Aborting" — and a port nothing ever listens on looks exactly like a
// server that is still starting. Without the exit channel the operator watches
// "starting up…" for the whole timeout before being told anything, which is not
// waiting. It is a hang with a deadline.
//
// A nil exited never fires, so WaitReady is the same call with nothing to watch.
func WaitReadyOrExit(ctx context.Context, probe Probe, exited <-chan struct{}, interval time.Duration) error {
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}

	var last error

	for {
		if err := probe(ctx); err == nil {
			return nil
		} else {
			last = err
		}

		timer := time.NewTimer(interval)

		select {
		case <-ctx.Done():
			timer.Stop()

			return fmt.Errorf("the database server did not become ready: %w (last attempt: %v)",
				ctx.Err(), last)
		case <-exited:
			timer.Stop()

			return fmt.Errorf("the database server stopped before it became ready (last attempt: %v)", last)
		case <-timer.C:
		}
	}
}

// Process is a running server.
type Process interface {
	// Done is closed when the process has exited.
	Done() <-chan struct{}

	// Err is why it exited, once Done is closed. Nil for a clean stop.
	Err() error

	// Stop asks the server to shut down and waits for it to go.
	Stop(ctx context.Context) error
}

// Launcher starts a server process. It exists so the supervisor's lifecycle can
// be tested without spawning anything.
type Launcher interface {
	Start(cfg Config) (Process, error)
}

// Supervisor owns one server's lifetime.
type Supervisor struct {
	launcher Launcher
	proc     Process
}

// NewSupervisor builds a supervisor over a launcher.
func NewSupervisor(l Launcher) *Supervisor { return &Supervisor{launcher: l} }

// EnsureStarted starts the server unless it is already running.
//
// Idempotent on purpose: boot happens on every launch, and a second start
// against the same data directory is how a client's books get corrupted — with
// no immediate symptom. A failed start leaves nothing running, so the next
// attempt tries again rather than believing a server is up.
func (s *Supervisor) EnsureStarted(_ context.Context, cfg Config) error {
	if s.Running() {
		return nil
	}

	proc, err := s.launcher.Start(cfg)
	if err != nil {
		return fmt.Errorf("start the database server: %w", err)
	}

	s.proc = proc

	return nil
}

// Running reports whether the server is up.
func (s *Supervisor) Running() bool {
	if s.proc == nil {
		return false
	}

	select {
	case <-s.proc.Done():
		return false
	default:
		return true
	}
}

// Done is closed when the running server exits, or nil when none is running.
//
// A caller waiting for a server to become ready needs this to tell "still
// starting" from "already dead" — the two look identical from a port that
// nothing is listening on.
func (s *Supervisor) Done() <-chan struct{} {
	if s.proc == nil {
		return nil
	}

	return s.proc.Done()
}

// Stop shuts the server down. Stopping nothing is not an error.
//
// Shutdown runs on every exit path, including the ones where the server never
// started. A shutdown that can fail on a machine where nothing went wrong is a
// shutdown that hides the real error.
func (s *Supervisor) Stop(ctx context.Context) error {
	if s.proc == nil {
		return nil
	}

	proc := s.proc
	s.proc = nil

	if err := proc.Stop(ctx); err != nil {
		return fmt.Errorf("stop the database server: %w", err)
	}

	return nil
}
