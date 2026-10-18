package shell_test

import (
	"strings"
	"testing"

	"fingo/internal/ui/shell"
)

// TestHeaderText_AnswersTheThreeQuestions — the header sits above every figure
// in the application, and there are exactly three things a figure needs before
// it means anything: whose books, which period, and whether that period can
// still be posted to.
//
// A number without them is a number nobody can act on, and worse, a number
// somebody will act on wrongly.
func TestHeaderText_AnswersTheThreeQuestions(t *testing.T) {
	tests := []struct {
		name   string
		client string
		period string
		locked bool
		want   []string
		absent []string
	}{
		{
			name: "nothing open yet",
			want: []string{"Chưa mở sổ khách hàng nào"},
		},
		{
			name:   "a client but no period",
			client: "Công ty Bình Minh",
			want:   []string{"Công ty Bình Minh"},
			// A separator with nothing after it reads as a truncated screen.
			absent: []string{"·"},
		},
		{
			name:   "a client in an open period",
			client: "Công ty Bình Minh",
			period: "FY2026",
			want:   []string{"Công ty Bình Minh", "FY2026", "đang mở"},
		},
		{
			name:   "a client in a locked period",
			client: "Công ty Bình Minh",
			period: "FY2026",
			locked: true,
			want:   []string{"Công ty Bình Minh", "FY2026", "đã khóa sổ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shell.HeaderText(tt.client, tt.period, tt.locked)

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("header %q does not say %q", got, want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(got, absent) {
					t.Errorf("header %q contains %q, which has nothing to introduce", got, absent)
				}
			}
		})
	}
}

// TestHeaderText_ALockedPeriodSaysSoRatherThanOpening — the distinction is the
// whole point of the lock. "Đang mở" on a closed period tells an operator they
// can post to it, and they will try, and the refusal will look like a bug.
func TestHeaderText_ALockedPeriodSaysSoRatherThanOpening(t *testing.T) {
	locked := shell.HeaderText("Công ty Bình Minh", "FY2026", true)
	open := shell.HeaderText("Công ty Bình Minh", "FY2026", false)

	if locked == open {
		t.Fatal("a locked period and an open one read the same")
	}
	if strings.Contains(locked, "đang mở") {
		t.Errorf("a locked period reads %q, which says it is open", locked)
	}
}
