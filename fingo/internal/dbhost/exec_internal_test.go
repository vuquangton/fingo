package dbhost

import (
	"io"
	"testing"
)

// TestCommand_GivesTheServerNoConsole — slice P1.3, and a defect that shipped.
//
// mariadbd is a console application. FinGo is built with -H windowsgui and so
// has no console for the child to inherit, and Windows therefore creates one —
// a black window captioned with the server's path, sitting on top of the
// application and looking exactly like a crash.
//
// Redirecting stdout and stderr does not prevent it. Those decide where a
// process's output goes; they say nothing about whether it has a console.
// CREATE_NO_WINDOW is the flag that does.
//
// Pinned because it is one field on one struct, it has no visible effect in any
// test that does not launch a process, and deleting it looks like tidying up.
func TestCommand_GivesTheServerNoConsole(t *testing.T) {
	l := &ExecLauncher{}

	cmd := l.command(Config{
		MysqldExe: `C:\nowhere\mariadbd.exe`,
		Datadir:   `C:\nowhere\data`,
		Port:      3306,
	})

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil, so the server is started with no CREATE_NO_WINDOW and gets a console window")
	}

	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Errorf("CreationFlags = %#x, want CREATE_NO_WINDOW (%#x) set", cmd.SysProcAttr.CreationFlags, createNoWindow)
	}
}

// TestCommand_DiscardsTheServersOutput — stdout and stderr must be io.Discard
// rather than nil.
//
// Nil means "share the parent's", which is the thing being avoided: a server
// writing into the application's terminal sprays its log across it. These two
// fields and CREATE_NO_WINDOW are the same decision made twice.
func TestCommand_DiscardsTheServersOutput(t *testing.T) {
	l := &ExecLauncher{}

	cmd := l.command(Config{MysqldExe: `C:\nowhere\mariadbd.exe`, Datadir: `C:\nowhere\data`})

	if cmd.Stdout != io.Discard {
		t.Error("Stdout is not discarded, so the server shares whatever console it is given")
	}
	if cmd.Stderr != io.Discard {
		t.Error("Stderr is not discarded, so the server's errors share whatever console it is given")
	}
}
