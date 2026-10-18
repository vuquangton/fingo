package client_test

import (
	"context"
	"errors"
	"testing"

	"fingo/internal/client"
	"fingo/internal/domain/gl"
)

// errNoDatabase stands in for a provisioner that cannot reach the client's
// database.
var errNoDatabase = errors.New("no database for this client")

// mustClient builds a client or fails the test.
func mustClient(t *testing.T, id, name, taxID string) client.Client {
	t.Helper()

	c, err := client.NewClient(id, name, taxID, day1)
	if err != nil {
		t.Fatalf("NewClient(%q): %v", id, err)
	}

	return c
}

// mustScope builds a scope or fails the test.
func mustScope(t *testing.T, id string) client.Scope {
	t.Helper()

	s, err := client.ScopeFor(id)
	if err != nil {
		t.Fatalf("ScopeFor(%q): %v", id, err)
	}

	return s
}

// registry returns a registry and the chart store behind it.
func registry(t *testing.T) (*client.MemoryRegistry, *client.MemoryChartStore) {
	t.Helper()

	charts := client.NewMemoryChartStore()

	return client.NewMemoryRegistry(client.NewChartProvisioner(charts)), charts
}

// register puts two clients in a registry, which is the state every test about
// isolation starts from.
func register(t *testing.T) (*client.MemoryRegistry, *client.MemoryChartStore, client.Client, client.Client) {
	t.Helper()

	reg, charts := registry(t)
	ctx := context.Background()

	a := mustClient(t, "c-a", "Công ty Bình Minh", "0101234567")
	b := mustClient(t, "c-b", "Công ty Đông Á", "0107654321")

	for _, c := range []client.Client{a, b} {
		if err := reg.Create(ctx, c); err != nil {
			t.Fatalf("Create(%s): %v", c.ID, err)
		}
	}

	return reg, charts, a, b
}

func TestMemoryRegistry_CreatesReadsUpdatesAndCloses(t *testing.T) {
	ctx := context.Background()
	reg, _, a, b := register(t)

	got, err := reg.Get(ctx, "c-a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != a.Name || got.TaxID != a.TaxID || !got.Active {
		t.Fatalf("round trip = %+v", got)
	}

	listed, err := reg.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("clients = %d, want 2", len(listed))
	}
	// The directory's order, not the map's: Bình Minh sorts before Đông Á.
	if listed[0].ID != "c-a" || listed[1].ID != "c-b" {
		t.Errorf("list order = %s, %s; want c-a then c-b", listed[0].ID, listed[1].ID)
	}

	// Update carries what changed. It must not restamp the creation instant or
	// reopen a client somebody closed.
	renamed := client.Client{ID: b.ID, Name: "Công ty Đông Á (mới)", TaxID: b.TaxID}
	if err := reg.Update(ctx, renamed); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := reg.SetActive(ctx, b.ID, false); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	got, err = reg.Get(ctx, b.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if got.Name != "Công ty Đông Á (mới)" {
		t.Errorf("name = %q, want the new name", got.Name)
	}
	if !got.CreatedAt.Equal(b.CreatedAt) {
		t.Errorf("created at = %s, want the original %s", got.CreatedAt, b.CreatedAt)
	}
	if got.Active {
		t.Error("a client closed by SetActive is still active")
	}

	// Closing keeps the books: the client is still there to be found.
	if _, err := reg.Get(ctx, b.ID); err != nil {
		t.Errorf("a closed client was lost: %v", err)
	}
	if err := reg.SetActive(ctx, "c-nobody", false); !errors.Is(err, client.ErrUnknownClient) {
		t.Errorf("SetActive on an unknown client = %v, want ErrUnknownClient", err)
	}
	if err := reg.Update(ctx, mustClient(t, "c-nobody", "Nobody", "")); !errors.Is(err, client.ErrUnknownClient) {
		t.Errorf("Update of an unknown client = %v, want ErrUnknownClient", err)
	}
}

// One company, one registration. Two rows for one tax code is one company's
// books split in two, and nothing later can tell which half is which.
func TestMemoryRegistry_RefusesADuplicateIdOrTaxCode(t *testing.T) {
	ctx := context.Background()
	reg, _, a, _ := register(t)

	if err := reg.Create(ctx, mustClient(t, a.ID, "Another Company", "")); !errors.Is(err, client.ErrClientExists) {
		t.Errorf("duplicate id = %v, want ErrClientExists", err)
	}
	if err := reg.Create(ctx, mustClient(t, "c-c", "Same Tax Code Ltd", a.TaxID)); !errors.Is(err, client.ErrTaxIDTaken) {
		t.Errorf("duplicate tax code = %v, want ErrTaxIDTaken", err)
	}

	// A tax code also cannot be moved onto a company that already holds it.
	other := mustClient(t, "c-c", "Công ty Mới", "0109999999")
	if err := reg.Create(ctx, other); err != nil {
		t.Fatalf("Create: %v", err)
	}
	other.TaxID = a.TaxID
	if err := reg.Update(ctx, other); !errors.Is(err, client.ErrTaxIDTaken) {
		t.Errorf("moving a tax code onto a taken one = %v, want ErrTaxIDTaken", err)
	}
}

// A client may be engaged before its paperwork is to hand. Two clients with no
// tax code yet are two clients, not a duplicate.
func TestMemoryRegistry_AllowsClientsWithNoTaxCode(t *testing.T) {
	ctx := context.Background()
	reg, _ := registry(t)

	if err := reg.Create(ctx, mustClient(t, "c-1", "First", "")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := reg.Create(ctx, mustClient(t, "c-2", "Second", "")); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

// Registration is one act. A client whose chart of accounts could not be
// installed cannot keep books, so it is not a client.
func TestMemoryRegistry_DoesNotRegisterAClientWhoseBooksCannotBeSetUp(t *testing.T) {
	ctx := context.Background()

	broken := client.ProvisionerFunc(func(context.Context, client.Client) error {
		return errNoDatabase
	})
	reg := client.NewMemoryRegistry(broken)

	err := reg.Create(ctx, mustClient(t, "c-1", "Công ty Bình Minh", "0101234567"))
	if err == nil {
		t.Fatal("a client was registered even though its books could not be set up")
	}
	if !errors.Is(err, errNoDatabase) {
		t.Fatalf("Create = %v, want the provisioner's failure wrapped", err)
	}

	if _, err := reg.Get(ctx, "c-1"); !errors.Is(err, client.ErrUnknownClient) {
		t.Fatalf("the half-registered client is in the registry: %v", err)
	}
}

// The test the objective names: a new client is given the standard chart of
// accounts, and it is the client's own — not a chart another client can change.
func TestChartProvisioner_GivesANewClientTheDefaultChartOfAccounts(t *testing.T) {
	ctx := context.Background()
	_, charts, a, b := register(t)

	want := len(client.DefaultChart())
	if want == 0 {
		t.Fatal("the default chart of accounts is empty")
	}

	chartA, err := charts.Chart(ctx, mustScope(t, a.ID))
	if err != nil {
		t.Fatalf("Chart(c-a): %v", err)
	}

	// Every account of the default chart is in the client's, so the seeded
	// chart is the whole of Phụ lục 1 and not an excerpt of it.
	for _, account := range client.DefaultChart() {
		if _, ok := chartA.ByCode(account.Code); !ok {
			t.Errorf("TK %s is missing from the seeded chart", account.Code)
		}
	}
	if _, ok := chartA.ByCode("9999"); ok {
		t.Error("TK 9999 is in the seeded chart and is not an account of TT133")
	}

	// A real account from Phụ lục 1, with its hierarchy: TK 1111 sits under
	// TK 111, so 111 is a heading and 1111 is what a posting names.
	cash, ok := chartA.ByCode("1111")
	if !ok {
		t.Fatal("TK 1111 is missing from the seeded chart")
	}
	if cash.ParentCode != "111" || cash.Name != "Tiền Việt Nam" {
		t.Errorf("TK 1111 = %+v, want Tiền Việt Nam under 111", cash)
	}
	if !chartA.IsPostable("1111") {
		t.Error("TK 1111 should be postable")
	}
	if chartA.IsPostable("111") {
		t.Error("TK 111 is a heading and should not be postable")
	}

	chartB, err := charts.Chart(ctx, mustScope(t, b.ID))
	if err != nil {
		t.Fatalf("Chart(c-b): %v", err)
	}
	if chartA == chartB {
		t.Fatal("two clients share one chart of accounts")
	}
	if _, err := charts.Chart(ctx, mustScope(t, "c-nobody")); !errors.Is(err, client.ErrUnknownClient) {
		t.Errorf("Chart for an unregistered client = %v, want ErrUnknownClient", err)
	}
}

// A chart is financial configuration and is scoped like everything else.
func TestMemoryChartStore_RefusesAnUnscopedChart(t *testing.T) {
	ctx := context.Background()
	charts := client.NewMemoryChartStore()

	var zero client.Scope

	if err := charts.SaveChart(ctx, zero, client.DefaultChart()); !errors.Is(err, client.ErrNoScope) {
		t.Errorf("SaveChart with no scope = %v, want ErrNoScope", err)
	}
	if _, err := charts.Chart(ctx, zero); !errors.Is(err, client.ErrNoScope) {
		t.Errorf("Chart with no scope = %v, want ErrNoScope", err)
	}
}

// A chart that does not hold together is refused before it is filed, not
// discovered at the first posting.
func TestMemoryChartStore_RefusesAChartThatIsNotUsable(t *testing.T) {
	ctx := context.Background()
	charts := client.NewMemoryChartStore()

	broken := []gl.Account{
		// 1111 claims a parent that is not in the chart.
		{Code: "1111", Name: "Tiền Việt Nam", ParentCode: "111", Type: gl.AccountTypeAsset, Nature: gl.NatureDebit},
	}
	if err := charts.SaveChart(ctx, mustScope(t, "c-1"), broken); err == nil {
		t.Fatal("a chart referencing a missing parent was accepted")
	}

	// Nothing was filed by the refused write.
	if _, err := charts.Chart(ctx, mustScope(t, "c-1")); !errors.Is(err, client.ErrUnknownClient) {
		t.Errorf("a refused chart was filed anyway: %v", err)
	}
}
