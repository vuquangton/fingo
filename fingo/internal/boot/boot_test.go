package boot_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"fingo/internal/boot"
)

// fakeProvisioner records what was asked of it, and answers from a scripted
// state. Nothing here touches a database, which is the point: the decision
// sequence is what is under test, not the work each step does.
type fakeProvisioner struct {
	hasData   bool
	hasDB     bool
	schemaVer int64
	hasChart  bool
	failAt    string
	calls     []string
}

func (f *fakeProvisioner) note(name string) error {
	f.calls = append(f.calls, name)

	if f.failAt == name {
		return errors.New(name + " failed")
	}

	return nil
}

func (f *fakeProvisioner) DataDirectoryExists() bool { return f.hasData }

func (f *fakeProvisioner) CreateDataDirectory(context.Context) error {
	if err := f.note("CreateDataDirectory"); err != nil {
		return err
	}
	f.hasData = true

	return nil
}

func (f *fakeProvisioner) StartServer(context.Context) error { return f.note("StartServer") }

func (f *fakeProvisioner) DatabaseExists(context.Context) (bool, error) {
	if err := f.note("DatabaseExists"); err != nil {
		return false, err
	}

	return f.hasDB, nil
}

func (f *fakeProvisioner) CreateDatabase(context.Context) error {
	if err := f.note("CreateDatabase"); err != nil {
		return err
	}
	f.hasDB = true

	return nil
}

func (f *fakeProvisioner) SchemaVersion(context.Context) (int64, error) {
	if err := f.note("SchemaVersion"); err != nil {
		return 0, err
	}

	return f.schemaVer, nil
}

func (f *fakeProvisioner) Migrate(context.Context) error {
	if err := f.note("Migrate"); err != nil {
		return err
	}
	f.schemaVer = 15

	return nil
}

func (f *fakeProvisioner) ChartExists(context.Context) (bool, error) {
	if err := f.note("ChartExists"); err != nil {
		return false, err
	}

	return f.hasChart, nil
}

func (f *fakeProvisioner) SeedChart(context.Context) error {
	if err := f.note("SeedChart"); err != nil {
		return err
	}
	f.hasChart = true

	return nil
}

// TestEnsure_ProvisionsAFreshInstallation — the first run, in the only order
// that works.
//
// The order is not a preference. A data directory must exist before a server can
// open it; a server must be up before a database can be created in it; a
// database must exist before migrations can run; and migrations must have run
// before the chart of accounts can be seeded into their tables. Any other order
// fails, and each failure looks like a different problem.
func TestEnsure_ProvisionsAFreshInstallation(t *testing.T) {
	p := &fakeProvisioner{}

	if err := boot.Ensure(t.Context(), p); err != nil {
		t.Fatalf("Ensure on a fresh installation: %v", err)
	}

	want := []string{
		"CreateDataDirectory",
		"StartServer",
		"DatabaseExists",
		"CreateDatabase",
		"SchemaVersion",
		"Migrate",
		"ChartExists",
		"SeedChart",
	}

	if !reflect.DeepEqual(p.calls, want) {
		t.Errorf("the provisioning order was\n  %v\nwant\n  %v", p.calls, want)
	}
}

// TestEnsure_LeavesAFinishedInstallationAlone — the second and every later run.
//
// This is the whole reason the sequence probes before it acts: a launch that
// re-seeds a chart or re-runs migrations is a launch that can destroy books it
// was supposed to open. Everything already there is left exactly as it is.
func TestEnsure_LeavesAFinishedInstallationAlone(t *testing.T) {
	p := &fakeProvisioner{hasData: true, hasDB: true, schemaVer: 15, hasChart: true}

	if err := boot.Ensure(t.Context(), p); err != nil {
		t.Fatalf("Ensure on a finished installation: %v", err)
	}

	want := []string{"StartServer", "DatabaseExists", "SchemaVersion", "ChartExists"}

	if !reflect.DeepEqual(p.calls, want) {
		t.Errorf("a finished installation was touched:\n  %v\nwant only probes plus the start:\n  %v", p.calls, want)
	}
}

// TestEnsure_ResumesFromWhereverItStopped — a first run that failed half way
// must be resumable, because the failure will be noticed and the application
// restarted rather than uninstalled.
func TestEnsure_ResumesFromWhereverItStopped(t *testing.T) {
	tests := []struct {
		name  string
		state *fakeProvisioner
		want  []string
	}{
		{
			name:  "the data directory is there but nothing else is",
			state: &fakeProvisioner{hasData: true},
			want: []string{"StartServer", "DatabaseExists", "CreateDatabase",
				"SchemaVersion", "Migrate", "ChartExists", "SeedChart"},
		},
		{
			name:  "the database exists but has no schema",
			state: &fakeProvisioner{hasData: true, hasDB: true},
			want: []string{"StartServer", "DatabaseExists", "SchemaVersion",
				"Migrate", "ChartExists", "SeedChart"},
		},
		{
			name:  "everything but the chart",
			state: &fakeProvisioner{hasData: true, hasDB: true, schemaVer: 15},
			want:  []string{"StartServer", "DatabaseExists", "SchemaVersion", "ChartExists", "SeedChart"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := boot.Ensure(t.Context(), tt.state); err != nil {
				t.Fatalf("Ensure: %v", err)
			}

			if !reflect.DeepEqual(tt.state.calls, tt.want) {
				t.Errorf("the sequence was\n  %v\nwant\n  %v", tt.state.calls, tt.want)
			}
		})
	}
}

// TestEnsure_StopsAtTheFirstFailure — a step that failed leaves everything after
// it undone, and the error says which step it was. Continuing past a failure
// would migrate a database that was never created, and report the resulting
// error instead of the real one.
func TestEnsure_StopsAtTheFirstFailure(t *testing.T) {
	p := &fakeProvisioner{failAt: "CreateDatabase"}

	err := boot.Ensure(t.Context(), p)

	if err == nil {
		t.Fatal("Ensure = nil, want the failing step's error")
	}

	last := p.calls[len(p.calls)-1]
	if last != "CreateDatabase" {
		t.Errorf("the last step attempted was %q, want CreateDatabase", last)
	}

	for _, after := range []string{"SchemaVersion", "Migrate", "ChartExists", "SeedChart"} {
		for _, called := range p.calls {
			if called == after {
				t.Errorf("%s ran after CreateDatabase failed", after)
			}
		}
	}
}

// TestEnsure_ReportsWhichStepFailed — the error has to name the step, because
// "provisioning failed" sends somebody to look at all of it.
func TestEnsure_ReportsWhichStepFailed(t *testing.T) {
	p := &fakeProvisioner{failAt: "Migrate"}

	err := boot.Ensure(t.Context(), p)
	if err == nil {
		t.Fatal("Ensure = nil, want an error")
	}

	if got := strings.ToLower(err.Error()); !strings.Contains(got, "migrat") {
		t.Errorf("error %q does not name the step that failed", err)
	}
}

// TestEnsure_ReportsAProbeThatFailed — a question that could not be answered is
// not the same as a step that failed, and the two need different fixes: one is
// "the server is not answering", the other is "the schema would not apply".
//
// A probe that fails must also stop the sequence. Carrying on would treat "I
// could not tell whether the database exists" as "it does not", and create one
// over the top of the client's books.
func TestEnsure_ReportsAProbeThatFailed(t *testing.T) {
	for _, probe := range []string{"DatabaseExists", "SchemaVersion", "ChartExists"} {
		t.Run(probe, func(t *testing.T) {
			p := &fakeProvisioner{
				hasData: true, hasDB: true, schemaVer: 15, hasChart: true,
				failAt: probe,
			}

			err := boot.Ensure(t.Context(), p)

			if err == nil {
				t.Fatalf("Ensure = nil, want the %s failure", probe)
			}
			if !strings.Contains(err.Error(), probe) {
				t.Errorf("error %q does not name %s", err, probe)
			}
			if last := p.calls[len(p.calls)-1]; last != probe {
				t.Errorf("the sequence continued past %s, reaching %s", probe, last)
			}
		})
	}
}
