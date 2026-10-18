package shell

import (
	"fmt"
	"os"
	"runtime/debug"
)

// guard runs a widget callback so that a panic inside it does not take the
// application down.
//
// This is not defensive programming for its own sake. A panic that escapes a
// Windows callback cannot be unwound through the frames above it — they are C,
// and the Go runtime has no way to walk past them — so the process is aborted
// where it stands. Worse, the runtime prints the panic to file descriptor 2, and
// a windowsgui application has no descriptor 2: the application exits with code
// 2 and says nothing, anywhere, to anyone.
//
// That is the worst possible failure for an accounting application: an
// accountant loses whatever they had typed, and there is no record of why. The
// guard costs a deferred call and turns it into a sentence on the status bar and
// a stack trace in the log.
//
// It is deliberately around the *entry points* — the rail, the table, the form —
// because those are the frames the toolkit calls into. A helper called from one
// of them is already inside the guard.
func (s *Shell) guard(what string, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		// os.Stderr is the log file — see cmd/fingo. The runtime's own printer
		// does not go through it, which is exactly the hole this fills.
		fmt.Fprintf(os.Stderr, "panic in %s: %v\n%s\n", what, r, debug.Stack())

		s.SetStatus(fmt.Sprintf("Lỗi: %v", r))
		s.showMessage(fmt.Sprintf("Có lỗi khi %s.\n\n%v\n\nFinGo vẫn đang chạy; chi tiết đã ghi vào fingo.log.", what, r))
	}()

	f()
}

// guardRail wraps the rail's selection handler.
func (s *Shell) guardRail() { s.guard("chọn mục", s.showSelected) }

// guardDrill wraps the table's activation handler.
func (s *Shell) guardDrill() { s.guard("mở dòng", s.drill) }

// guardCommit wraps a cell being committed.
func (s *Shell) guardCommit(f func()) { s.guard("nhập ô", f) }

// guardPost wraps the form being posted.
func (s *Shell) guardPost(f func()) { s.guard("ghi sổ", f) }
