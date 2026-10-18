package dbhost

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// InitConfig describes the one-time creation of a data directory.
type InitConfig struct {
	InstallExe string
	Datadir    string
}

// Args is the installer's command line.
//
// Deliberately no `--service`. That would register a Windows service: it needs
// administrator rights and it changes the machine. FinGo runs its server as a
// child process of the application instead, so closing the application closes
// the server and uninstalling is deleting a directory. A silent system-wide
// change is the opposite of a one-click install.
func (c InitConfig) Args() []string {
	return []string{"--datadir=" + c.Datadir}
}

// Init creates a data directory that did not exist.
//
// The installer's own output is carried into the error, because when this step
// fails it is the only thing that says why — a permissions problem and a
// malformed path look identical from here.
func Init(ctx context.Context, c InitConfig) error {
	if err := os.MkdirAll(filepath.Dir(c.Datadir), 0o755); err != nil {
		return fmt.Errorf("create the server directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.InstallExe, c.Args()...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initialise the data directory: %w\n%s", err, out)
	}

	return nil
}

// ExecLauncher starts the server as a child process of this application, and
// confines it to this application's lifetime.
//
// The confinement is not a nicety. A server that outlives the application holds
// the data directory open, and the next launch then fails with "InnoDB: The data
// file './ibdata1' must be writable" — the application cannot be started again
// until somebody finds the process in Task Manager. A clean exit stops the
// server properly, but a clean exit is not something to depend on.
type ExecLauncher struct {
	// job holds the servers this launcher started. Created on the first start,
	// and deliberately never closed: it closes when this process ends, which is
	// the whole mechanism.
	job *Job
}

// Start launches the server and returns a handle to it.
func (l *ExecLauncher) Start(cfg Config) (Process, error) {
	cmd := l.command(cfg)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", cfg.MysqldExe, err)
	}

	// Before anything else can fail. A server that escapes before it is confined
	// is a server that can lock a client out of their own books.
	if err := l.confine(cmd.Process); err != nil {
		_ = cmd.Process.Kill()

		return nil, err
	}

	p := &execProcess{
		cmd:   cmd,
		done:  make(chan struct{}),
		admin: cfg.AdminExe,
		port:  cfg.Port,
	}

	// One goroutine, with a clear exit: it returns when the server does.
	// Closing done publishes err safely — the close happens before any receive
	// from done returns, so Err() is readable without a lock.
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()

	return p, nil
}

// command builds the server's process.
//
// Separate from Start so that the two decisions here can be asserted without
// launching anything.
func (l *ExecLauncher) command(cfg Config) *exec.Cmd {
	cmd := exec.Command(cfg.MysqldExe, cfg.Args()...)

	// The server's output is discarded: it writes to the log file named in
	// Config, and a child sharing this process's console would otherwise spray
	// server log lines across the application's terminal.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	// And it must not be given a console of its own.
	//
	// mariadbd is a console application. The shipped application is built with
	// -H windowsgui and so has no console for the child to inherit, and Windows
	// therefore creates one: a black window captioned with the server's path,
	// sitting on top of the application, looking exactly like a crash. Found by
	// screenshotting the whole screen rather than the application's own window.
	//
	// Redirecting stdout and stderr does not prevent this — those decide where a
	// process's output goes, not whether it has a console at all.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}

	return cmd
}

// createNoWindow is CREATE_NO_WINDOW: a console application started with it runs
// with no console window.
const createNoWindow = 0x08000000

// confine puts a freshly started server into this application's job object.
func (l *ExecLauncher) confine(p *os.Process) error {
	if l.job == nil {
		job, err := NewJob()
		if err != nil {
			return fmt.Errorf(
				"a database server that cannot be confined to this process's lifetime is a server that can lock a client out of their books: %w",
				err)
		}

		l.job = job
	}

	if err := l.job.Assign(p); err != nil {
		return fmt.Errorf("confine the database server to this process's lifetime: %w", err)
	}

	return nil
}

// execProcess is a running server process.
type execProcess struct {
	cmd   *exec.Cmd
	done  chan struct{}
	err   error
	admin string
	port  int
}

func (p *execProcess) Done() <-chan struct{} { return p.done }

func (p *execProcess) Err() error { return p.err }

// Stop asks the server to shut down, then insists.
//
// A clean shutdown matters more here than it looks: an InnoDB data directory
// that is killed on every exit works, but it recovers on every launch, and a
// client's books are not a place to accept "it recovers" as the normal path.
// The kill is the backstop for a server that will not take the hint.
//
// The caller's cancellation is deliberately detached from. Shutting the server
// down is what happens *because* the application is ending, so a context that is
// already cancelled is the normal case, not an error — and honouring it here
// would abandon a server still holding the data directory open.
func (p *execProcess) Stop(ctx context.Context) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
	defer cancel()

	if p.admin != "" {
		shutdown := exec.CommandContext(stopCtx, p.admin,
			"--host=127.0.0.1",
			fmt.Sprintf("--port=%d", p.port),
			"--user=root",
			"shutdown")

		if _, err := shutdown.CombinedOutput(); err == nil {
			select {
			case <-p.done:
				return nil
			case <-stopCtx.Done():
				// Fall through to the kill.
			}
		}
	}

	if err := p.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill the database server: %w", err)
	}

	select {
	case <-p.done:
		return nil
	case <-stopCtx.Done():
		return stopCtx.Err()
	}
}
