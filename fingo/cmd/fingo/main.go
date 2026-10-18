// Command fingo is the FinGo desktop application.
//
// It owns four things and delegates everything else: the single-instance guard,
// the installation root, provisioning, and the window. The accounting engine is
// untouched by it, and the window is a widget layer over models that know
// nothing about it.
package main

// The manifest is compiled into a .syso and linked into this executable, and
// without it the application does not start at all — walk cannot create a
// tooltip, and Windows refuses to load the binary as soon as the manifest it
// carries is invalid.
//
// It is a build step and it lives here rather than in a README, so that
// `go generate ./cmd/fingo` is the whole of remembering it.
//
//go:generate rsrc -manifest app.manifest -arch amd64 -o rsrc.syso

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lxn/walk"

	"fingo/internal/appconfig"
	"fingo/internal/books"
	"fingo/internal/boot"
	"fingo/internal/dbhost"
	"fingo/internal/domain/gl"
	"fingo/internal/period"
	"fingo/internal/singleton"
	"fingo/internal/ui/accounts"
	"fingo/internal/ui/company"
	"fingo/internal/ui/home"
	"fingo/internal/ui/journal"
	ledgerui "fingo/internal/ui/ledger"
	"fingo/internal/ui/nav"
	"fingo/internal/ui/shell"
	"fingo/internal/ui/trialbalance"
	"fingo/internal/ui/voucher"
	"fingo/internal/usecase/ledger"
)

// provisionTimeout bounds the whole first run. Initialising a data directory and
// applying fifteen migrations takes about thirteen seconds on a quiet machine;
// five minutes is not a target, it is the point at which waiting has stopped
// being reasonable and the server log is the thing to read.
const provisionTimeout = 5 * time.Minute

// accountingYear is the calendar year the application is keeping books in.
//
// The same range openTheYear creates, so the drill-through reads exactly the
// period the report behind it was built from.
func accountingYear(now time.Time) (from, to time.Time) {
	return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(now.Year(), 12, 31, 0, 0, 0, 0, time.UTC)
}

// openTheYear gives a new client an accounting period.
//
// Nothing seeds one, and every report refuses without one: a balance sheet is a
// position at the end of a period, and there is no such thing as a position at
// the end of nothing. An application that installs itself, provisions its own
// database and then cannot answer a single question about the books it just
// created has not finished setting itself up.
//
// It creates nothing if the client already has periods, so it is safe to call on
// every launch.
func openTheYear(ctx context.Context, client *books.Books) (period.Period, error) {
	periods, err := client.Periods(ctx)
	if err != nil {
		return period.Period{}, fmt.Errorf("read the accounting periods: %w", err)
	}

	if len(periods) > 0 {
		return period.Period{}, nil
	}

	year := time.Now().Year()

	opening := period.Period{
		ID:        fmt.Sprintf("FY%d", year),
		StartDate: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	if err := client.CreatePeriod(ctx, opening); err != nil {
		return period.Period{}, fmt.Errorf("open the period %s: %w", opening.ID, err)
	}

	return opening, nil
}

// postEntry records a journal entry in the ledger.
//
// It is the last step of the write path: the grid has already refused anything
// it could refuse, and the ledger will refuse the rest. What is left here is
// naming the request — the ledger's idempotency key is the voucher's identity,
// so a retried post of the same document is the same document.
func postEntry(client *books.Books, grid voucher.EntryGrid, description string) (string, error) {
	if strings.TrimSpace(description) == "" {
		description = "Chứng từ nghiệp vụ khác"
	}

	now := time.Now()
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	id := uuid.NewString()

	v, err := grid.Voucher(id, date, description)
	if err != nil {
		return "", err
	}

	if err := client.Ledger().Post(context.Background(), ledger.Posting{
		Voucher:    v,
		At:         now.UTC(),
		By:         operator(),
		RequestKey: id,
	}); err != nil {
		return "", fmt.Errorf("ghi sổ: %w", err)
	}

	return v.VoucherNo(), nil
}

// operator is who the ledger records as having posted.
//
// The Windows account, because this is a single-user desktop application: there
// is no login, and inventing a name would be worse than recording the one the
// operating system already knows.
func operator() string {
	if name := os.Getenv("USERNAME"); name != "" {
		return name
	}

	return "ketoan"
}

// screenTimeout bounds one screen's read. A screen that cannot read its books in
// this long has a problem worth showing rather than one worth waiting on.
const screenTimeout = 30 * time.Second

// page is one screen's worth of already-decided content.
//
// By the time a page exists, every question about it has been answered: what it
// is called, what it says, what order its rows are in, how its money is written
// and what it says when it is empty. The shell draws it and decides nothing.
type page struct {
	title    string
	summary  string
	headings []string
	rows     [][]string
	empty    string
}

// screens maps a rail id to the function that reads and prepares its page.
//
// Each of these runs on a worker goroutine. That is safe precisely because the
// models contain no widgets: the only toolkit type in the whole path is the
// shell's table, and it is touched once, on the widget's thread, by ShowTable.
var screens = map[string]func(context.Context, *books.Books) (page, error){
	"today": func(ctx context.Context, client *books.Books) (page, error) {
		model := home.New(client, time.Now())

		p := page{title: model.Title()}

		if err := model.Load(ctx); err != nil {
			return p, fmt.Errorf("Không đọc được tình hình sổ sách.\n\n%w", err)
		}

		// The note rather than the summary: it carries the period, whether it is
		// closed, and what to do about it — the only line on the screen that
		// changes what the operator does next.
		p.summary = model.Note()
		p.headings, p.rows, p.empty = model.Table()

		return p, nil
	},

	"masterdata": func(ctx context.Context, client *books.Books) (page, error) {
		p := page{title: "Hệ thống tài khoản"}

		model := &accounts.Model{}
		if err := model.Load(ctx, client); err != nil {
			return p, fmt.Errorf("Không đọc được hệ thống tài khoản.\n\n%w", err)
		}

		p.summary = model.Summary()
		p.headings, p.rows, p.empty = model.Table()

		return p, nil
	},

	"books": func(ctx context.Context, client *books.Books) (page, error) {
		model := journal.New(client)

		p := page{title: model.Title()}

		from, to := accountingYear(time.Now())

		if err := model.Load(ctx, from, to); err != nil {
			return p, fmt.Errorf("Không đọc được sổ nhật ký chung.\n\n%w", err)
		}

		// The note, not the summary: it is where an unbalanced document would be
		// shouted about, and a journal that reported an impossible voucher as
		// ordinary would be reporting a book that cannot exist.
		p.summary = model.Note()
		p.headings, p.rows, p.empty = model.Table()

		return p, nil
	},

	"reports": func(ctx context.Context, client *books.Books) (page, error) {
		model := &trialbalance.Model{}

		p := page{title: model.Title()}

		if err := model.Load(ctx, client); err != nil {
			return p, fmt.Errorf("Không đọc được bảng cân đối số phát sinh.\n\n%w", err)
		}

		// The balance note goes in the status bar rather than in a column: on a
		// balanced book it is a reassurance, and on an unbalanced one it is the
		// only thing on the screen that matters.
		p.summary = model.Summary() + " · " + model.BalanceNote()
		p.headings, p.rows, p.empty = model.Table()

		return p, nil
	},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FinGo:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := installRoot()
	if err != nil {
		return err
	}

	// A desktop application has no console, so anything it fails at is a failure
	// nobody can find. Everything — the application's own messages and the
	// database server's — goes to one file under the installation root.
	//
	// This exists because of a real death that left no trace anywhere: the
	// window and its dialog vanished about twenty seconds after launch, the
	// process exited 0, stderr was empty and the event log said nothing. That is
	// unfindable, and the only cure is for the application to write down what it
	// was doing.
	if err := os.MkdirAll(root, 0o700); err == nil {
		if f, err := os.OpenFile(filepath.Join(root, "fingo.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			os.Stderr = f

			defer func() { _ = f.Close() }()

			fmt.Fprintln(os.Stderr, "--- FinGo starting, root:", root)
		}
	}

	// One file, under the installation root, so the whole installation is one
	// directory and uninstalling is deleting it.
	store := appconfig.NewStoreAt(filepath.Join(root, "config.json"))

	app := shell.New()

	// The books the screens read. Held in an atomic pointer because it is
	// written by the provisioning goroutine and read by the rail's selection
	// handler on the widget's thread, and those are different threads.
	var held atomic.Pointer[books.Books]

	// And the chart, for the same reason: the entry form has to know which
	// accounts a movement can be posted to, and that answer is read once and
	// then used on the widget's thread.
	var chart atomic.Pointer[gl.ChartOfAccounts]

	// openEntryForm shows the journal-entry screen.
	//
	// A fresh grid each time, from the chart read at startup: an abandoned entry
	// must not reappear under the next voucher.
	openEntryForm := func(client *books.Books) {
		accounts := chart.Load()
		if accounts == nil {
			app.ShowMessage("Chưa đọc được hệ thống tài khoản, nên chưa biết tài khoản nào hạch toán được.")

			return
		}

		grid := voucher.NewEntryGrid(accounts, 15)

		app.ShowEntryForm(grid, func(g voucher.EntryGrid, description string) error {
			// On the widget's thread, because the form is mid-callback and there
			// is no way to hand control back to it. One voucher insert is
			// milliseconds; anything slower would belong on a worker.
			voucherNo, err := postEntry(client, g, description)
			if err != nil {
				return err
			}

			app.EntryRecorded(voucherNo)
			app.SetStatus("Đã ghi sổ " + voucherNo)

			return nil
		})

		app.SetStatus("Chứng từ nghiệp vụ khác — Tab để qua ô, F8 để ghi sổ")
	}

	// openScreen loads a rail entry's screen and hands the shell its table.
	//
	// The load runs on a worker, not on the widget's thread, and that is the
	// whole reason the screens are toolkit-free models: everything except the
	// final hand-off — the read, the ordering, the filtering, the formatting,
	// the wording — happens where a slow query cannot freeze the window. Only
	// ShowTable touches a widget.
	// Which screen the content area is showing.
	//
	// Widget-thread only, and deliberately a plain string: it is written by the
	// rail's selection handler and read by the table's activation handler, and
	// both run on the thread that owns the widgets. Guarding it would suggest a
	// race the toolkit's own threading rules do not allow.
	showing := ""

	// openAccountLedger is the far end of the traceability spine, reached by
	// opening a figure on a report.
	openAccountLedger := func(client *books.Books, account string) {
		app.SetStatus("Sổ chi tiết " + account + " — đang tải…")

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), screenTimeout)
			defer cancel()

			from, to := accountingYear(time.Now())

			model := ledgerui.New(client, account)
			fmt.Fprintln(os.Stderr, "TRACE ledger loading", account)
			if err := model.Load(ctx, from, to); err != nil {
				app.Synchronize(func() {
					app.ShowMessage("Không đọc được sổ chi tiết.\n\n" + err.Error())
					app.SetStatus("Sổ chi tiết " + account + " — lỗi")
				})

				return
			}

			headings, rows, empty := model.Table()
			title, summary := model.Title(), model.Summary()

			app.Synchronize(func() {
				showing = "ledger"

				app.ShowTable(headings, rows, empty)
				app.SetStatus(title + " · " + summary)
			})
		}()
	}

	openScreen := func(id string) {
		showing = id

		client := held.Load()
		if client == nil {
			app.ShowMessage("Sổ sách chưa mở xong. Hãy đợi trong giây lát rồi chọn lại.")

			return
		}

		load, ok := screens[id]
		if !ok {
			// The entry form is not a table: it is a set of controls bound to a
			// grid, and it is shown directly rather than through a loader.
			if id == "general" {
				openEntryForm(client)

				return
			}

			// Reachable only if an id is added to the openable set without a
			// screen behind it. Saying so is better than a blank pane.
			app.ShowMessage("Màn hình " + id + " chưa được dựng.")

			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), screenTimeout)
			defer cancel()

			page, err := load(ctx, client)

			app.Synchronize(func() {
				if err != nil {
					app.ShowMessage(err.Error())
					app.SetStatus(page.title + " — lỗi")

					return
				}

				app.ShowTable(page.headings, page.rows, page.empty)
				app.SetStatus(page.title + " · " + page.summary)
			})
		}()
	}

	// The window exists before anything else can go wrong, because the first
	// thing that can go wrong is a second copy — and telling somebody that needs
	// somewhere to say it.
	if err := app.Create(shell.Options{
		Title:  "FinGo — SME Accounting (TT133)",
		Rail:   nav.BuildTree(nav.Rail()),
		Open:   openable{"today": true, "masterdata": true, "books": true, "reports": true, "general": true},
		Status: "đang khởi động…",
		OnOpen: openScreen,
		OnDrill: func(key string) {
			client := held.Load()
			if client == nil {
				return
			}

			// Only the trial balance has anywhere to drill to so far. The key of
			// one of its rows is an account number, and what is behind it is the
			// vouchers that put the figure there.
			if showing != "reports" {
				return
			}

			openAccountLedger(client, key)
		},
	}); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "window created")

	// One copy at a time. Two would start two database servers against one data
	// directory; MariaDB refuses the second, so the books are safe either way,
	// but the operator would get a raw server error instead of being told FinGo
	// is already open.
	//
	// The guard is named after the installation root, not the application, so
	// two installations on one machine are two installations — each guarding its
	// own books. NameFor turns the root into something a kernel object may be
	// called; passing the path itself fails at CreateMutex with an error about
	// paths.
	guard, err := singleton.Acquire(singleton.NameFor(root))
	if err != nil {
		if errors.Is(err, singleton.ErrAlreadyRunning) {
			fmt.Fprintln(os.Stderr, "another copy is already running; exiting without starting the server")

			walk.MsgBox(app.Window(), "FinGo đang chạy",
				"Một bản FinGo khác đã mở trên máy này.\n\nHãy dùng cửa sổ đang mở.",
				walk.MsgBoxIconInformation)

			return nil
		}

		return err
	}
	defer func() { _ = guard.Release() }()

	fmt.Fprintln(os.Stderr, "guard acquired; provisioning")

	// Provisioning runs off the UI thread and reports back through Synchronize.
	// It takes seconds the first time, and the window has to stay alive while it
	// does — a splash screen that cannot be moved is a splash screen that looks
	// like a hang.
	//
	// Nothing here is configured by hand: the port is chosen, the paths are
	// derived from the root, and the only input is a directory.
	var (
		live    *boot.Live
		provErr error
		done    = make(chan struct{})
	)

	go func() {
		defer close(done)

		// A panic on a worker goroutine takes the whole application down, and a
		// windowsgui process has no descriptor 2 for the runtime to print it to.
		// Recovering here at least leaves the reason in the log.
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "panic while provisioning: %v\n%s\n", r, debug.Stack())

				app.Synchronize(func() {
					app.SetStatus("lỗi khi mở sổ sách")
					app.ShowMessage(fmt.Sprintf("Có lỗi khi mở sổ sách.\n\n%v", r))
				})
			}
		}()

		live, provErr = provision(root)
		if provErr != nil {
			// To the console as well as the dialog. A desktop application's
			// failures are the ones nobody can attach a debugger to, and a line
			// on stderr costs nothing and is the only thing left when the dialog
			// has been dismissed.
			fmt.Fprintln(os.Stderr, "FinGo:", provErr)

			app.Synchronize(func() {
				app.SetStatus("không mở được sổ sách")
				walk.MsgBox(app.Window(), "FinGo không mở được sổ sách",
					provErr.Error(), walk.MsgBoxIconError)
			})

			return
		}

		// Open the books once, for the life of the application. Provisioning is
		// finished by now, so the screens have something to read.
		client, booksErr := live.Books(context.Background())
		if booksErr != nil {
			fmt.Fprintln(os.Stderr, "FinGo:", booksErr)
			app.Synchronize(func() { app.SetStatus("không mở được sổ sách") })

			return
		}
		held.Store(client)

		if opened, err := openTheYear(context.Background(), client); err != nil {
			fmt.Fprintln(os.Stderr, "FinGo:", err)
		} else if opened.ID != "" {
			fmt.Fprintln(os.Stderr, "FinGo: opened the accounting period", opened.ID)
		}

		if accounts, err := client.Chart(context.Background()); err == nil {
			chart.Store(accounts)
		} else {
			fmt.Fprintln(os.Stderr, "FinGo: read the chart of accounts:", err)
		}

		// Everything that touches the window happens in ONE Synchronize, in
		// order, on the thread that owns the widgets.
		//
		// Two Synchronize calls with a value passed between them is a race, and
		// it was: askErr was set inside the first closure and read on this
		// goroutine after it, so a save that failed said nothing at all and the
		// only symptom was a company name that never appeared. One closure
		// cannot have that bug.
		app.Synchronize(func() {
			settings, err := store.Load()
			if err != nil && !errors.Is(err, appconfig.ErrNotConfigured) {
				fmt.Fprintln(os.Stderr, "FinGo: read the settings:", err)
				app.SetStatus("không đọc được cấu hình")

				return
			}

			// Record where this installation keeps its books. Not asked for and
			// not configurable — but the settings file is invalid without it,
			// so a first run that saved only the company would write nothing.
			settings.Connection = live.Connection()

			client := settings.Company.Name

			if strings.TrimSpace(client) == "" {
				var form company.Form

				profile, err := shell.AskCompany(app.Window(), &form)
				switch {
				case errors.Is(err, shell.ErrCompanyCancelled):
					// They chose to answer later. The books are open either way,
					// and a name is not needed to record anything — so this is
					// not a failure, and nagging would be rude.
				case err != nil:
					fmt.Fprintln(os.Stderr, "FinGo: ask the company:", err)
				default:
					settings.Company = appconfig.Company{
						Name:    profile.Name,
						TaxCode: profile.TaxCode,
					}
					client = profile.Name
				}
			}

			if strings.TrimSpace(client) != "" {
				if err := store.Save(settings); err != nil {
					fmt.Fprintln(os.Stderr, "FinGo: save the settings:", err)
				}
			}

			app.SetHeader(client, "", false)
			app.SetStatus("sẵn sàng")
		})
	}()

	app.Run()

	fmt.Fprintln(os.Stderr, "the window was closed; exiting")

	// The window is closed. The server is stopped and the pools given up, on a
	// context of our own: the application ending is why this runs, so a
	// cancelled context is the normal case rather than a reason to skip it.
	<-done
	if live != nil {
		_ = live.Close(context.Background())
	}

	return nil
}

// provision brings the installation up, creating whatever is missing.
func provision(root string) (*boot.Live, error) {
	ctx, cancel := context.WithTimeout(context.Background(), provisionTimeout)
	defer cancel()

	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find where FinGo is installed: %w", err)
	}

	binDir, err := dbhost.FindBinDir(root, filepath.Dir(executable))
	if err != nil {
		return nil, err
	}

	port, err := freePort()
	if err != nil {
		return nil, err
	}

	live, err := boot.NewLive(boot.LiveConfig{Root: root, BinDir: binDir, Port: port})
	if err != nil {
		return nil, err
	}

	if err := boot.Ensure(ctx, live); err != nil {
		return nil, err
	}

	return live, nil
}

// freePort finds a loopback port nobody is listening on.
//
// A fresh port each launch, because nothing outside this process needs to know
// it and a port written down is a port that is eventually occupied.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find a port for the database server: %w", err)
	}
	defer func() { _ = l.Close() }()

	return l.Addr().(*net.TCPAddr).Port, nil
}

// openable is what the shell can actually open.
//
// Empty in P0, and that is the honest answer: the window exists, the books are
// provisioned, and the modules that read them are not built. Every rail entry
// says so in words rather than opening an empty screen.
type openable map[string]bool

func (o openable) CanOpen(id string) bool { return o[id] }

// installRoot is where this installation keeps everything it owns.
//
// One directory, under the user's own profile, so uninstalling is deleting it
// and nothing is left behind in Program Files or in the registry.
//
// FINGO_ROOT moves it, which is what makes a portable install possible and what
// keeps a test run out of the real profile.
func installRoot() (string, error) {
	if override := os.Getenv("FINGO_ROOT"); override != "" {
		return filepath.Clean(override), nil
	}

	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set, so there is nowhere to keep the books")
	}

	return filepath.Join(base, "FinGo"), nil
}
