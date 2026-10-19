package books_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"fingo/internal/books"
	"fingo/internal/domain/gl"
	"fingo/internal/ledger"
	"fingo/internal/money"
	"fingo/internal/store"
	"fingo/internal/testdb"
)

func TestHandoverExportContainsThisClientOnly(t *testing.T) {
	cfg := testdb.Load(t)
	testdb.Open(t, cfg)

	st, err := store.Open(t.Context(), store.Config{DSN: cfg.DSN()})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	makeClient := func(name string) (books.Books, string) {
		id := newUUID(t)
		if _, err := st.Pool().Exec(t.Context(),
			`INSERT INTO clients (id, name, tax_id) VALUES ($1, $2, $3)`,
			id, name, "HO-"+id); err != nil {
			t.Fatalf("insert client: %v", err)
		}
		scope, err := store.NewScope(id)
		if err != nil {
			t.Fatalf("scope: %v", err)
		}
		if err := st.InClient(t.Context(), scope, func(tx pgx.Tx) error {
			_, err := tx.Exec(t.Context(), `SELECT provision_client_chart($1)`, id)
			return err
		}); err != nil {
			t.Fatalf("provision chart: %v", err)
		}
		return *books.Open(st, scope), id
	}

	first, firstID := makeClient("Khách thứ nhất")
	_, _ = makeClient("Khách thứ hai")

	if _, err := ledger.Post(t.Context(), st, mustScope(t, firstID), ledger.Request{
		ID:          newUUID(t),
		Description: "Bút toán của khách thứ nhất",
		Actor:       "ketoan",
		Date:        time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Type:        gl.VoucherTypeGeneral,
		Lines: []ledger.Line{
			{Account: "1111", Side: gl.SideDebit, Amount: money.FromInt(1_000_000)},
			{Account: "4111", Side: gl.SideCredit, Amount: money.FromInt(1_000_000)},
		},
	}); err != nil {
		t.Fatalf("post: %v", err)
	}

	var buf bytes.Buffer
	if err := first.Export(t.Context(), &buf); err != nil {
		t.Fatalf("export: %v", err)
	}

	archive, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open the archive: %v", err)
	}

	files := map[string]string{}
	for _, f := range archive.File {
		reader, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		files[f.Name] = string(content)
	}

	vouchers, ok := files["vouchers.csv"]
	if !ok {
		t.Fatalf("the export has no vouchers.csv: %v", keys(files))
	}
	if got := dataRows(vouchers); got != 1 {
		t.Fatalf("vouchers.csv has %d rows, want 1: the export leaked or lost rows", got)
	}

	accounts := files["accounts.csv"]
	if got := dataRows(accounts); got != 127 {
		t.Fatalf("accounts.csv has %d rows, want 127", got)
	}

	if strings.Contains(files["MANIFEST.json"], "Khách thứ hai") {
		t.Fatal("the manifest names another client")
	}

	var manifest struct {
		ClientID string           `json:"client_id"`
		Tables   map[string]int64 `json:"tables"`
	}
	if err := json.Unmarshal([]byte(files["MANIFEST.json"]), &manifest); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if manifest.ClientID != firstID {
		t.Fatalf("manifest client = %s, want %s", manifest.ClientID, firstID)
	}
	if manifest.Tables["vouchers"] != 1 {
		t.Fatalf("manifest vouchers = %d, want 1", manifest.Tables["vouchers"])
	}
}

func mustScope(t *testing.T, id string) store.Scope {
	t.Helper()

	scope, err := store.NewScope(id)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	return scope
}

// dataRows counts CSV lines after the header, ignoring a trailing newline.
func dataRows(csv string) int {
	lines := strings.Split(strings.TrimRight(csv, "\n"), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return 0
	}
	return len(lines) - 1
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
