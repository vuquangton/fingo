package shell

import "strings"

// HeaderText is what the header says.
//
// It answers the three questions a figure needs answered before it means
// anything — whose books, which period, and whether that period can still be
// posted to — and it is a pure function so that the answer is asserted rather
// than eyeballed. Everything else in this package is layout, and layout is not
// worth a test.
//
// A separator with nothing after it reads as a truncated screen, so the parts
// are joined only when there is a part to join.
func HeaderText(client, period string, locked bool) string {
	client = strings.TrimSpace(client)
	period = strings.TrimSpace(period)

	if client == "" {
		return "Chưa mở sổ khách hàng nào"
	}
	if period == "" {
		return client
	}

	state := "đang mở"
	if locked {
		// The distinction is the whole point of the lock. "Đang mở" on a closed
		// period tells an operator they can post to it, and they will try, and
		// the refusal will look like a bug rather than like a lock.
		state = "đã khóa sổ"
	}

	return client + "   ·   " + period + "   ·   " + state
}
