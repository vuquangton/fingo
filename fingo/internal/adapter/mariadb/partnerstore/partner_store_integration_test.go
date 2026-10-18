package partnerstore_test

import (
	"context"
	"errors"
	"testing"

	"fingo/internal/adapter/mariadb/partnerstore"
	"fingo/internal/adapter/mariadb/testsupport"
	"fingo/internal/partner"
)

// newStore opens a migrated scratch database. It skips when no .env or no server
// is reachable, so a green run without a database proves nothing about this
// file.
func newStore(t *testing.T) *partnerstore.Store {
	t.Helper()

	cfg := testsupport.Config(t)
	admin := testsupport.Admin(t, cfg)
	db, _ := testsupport.MigratedDatabase(t, cfg, admin)

	return partnerstore.NewStore(db)
}

// The whole of the lifecycle, through a real table.
func TestStore_RoundTripsAPartner(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	p, err := partner.NewPartner("p-1", "Công ty Bình Minh", partner.TypeBoth, "0123456789", 30)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	warning, err := store.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if warning != nil {
		t.Fatalf("the first partner warns about %v", warning)
	}

	got, err := store.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != p.Name || got.Type != partner.TypeBoth || got.TaxID != "0123456789" ||
		got.PaymentTerms != 30 || !got.Active {
		t.Fatalf("round trip = %+v, want %+v", got, p)
	}

	// Update, then read it back.
	got.Type = partner.TypeSupplier
	got.PaymentTerms = 45
	got.Active = false
	if _, err := store.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	again, err := store.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if again.Type != partner.TypeSupplier || again.PaymentTerms != 45 || again.Active {
		t.Errorf("after update = %+v", again)
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("partners = %d, want 1", len(listed))
	}

	if err := store.Delete(ctx, "p-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, "p-1"); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Fatalf("Get after delete = %v, want ErrUnknownPartner", err)
	}
}

// A duplicate tax id is a warning, not a refusal — and the table has no UNIQUE
// key that would turn it into a failed insert. This is the case the schema was
// shaped for: a branch filing under its head office's code must be storable.
func TestStore_WarnsAboutADuplicateTaxIDInsteadOfRefusingIt(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	first, err := partner.NewPartner("p-1", "Công ty Bình Minh", partner.TypeCustomer, "0123456789", 30)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}
	if _, err := store.Create(ctx, first); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The same tax id, written the way somebody would type it, under a
	// different name and on the other side of the books.
	second, err := partner.NewPartner("p-2", "Bình Minh — chi nhánh", partner.TypeSupplier, "0123 456 789", 15)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	warning, err := store.Create(ctx, second)
	if err != nil {
		t.Fatalf("a duplicate tax id must not stop the insert: %v", err)
	}
	if warning == nil {
		t.Fatal("a duplicate tax id must be reported")
	}
	if warning.Code != partner.WarningDuplicateTaxID || warning.PartnerID != "p-1" {
		t.Errorf("warning = %+v, want the duplicate-tax-id warning naming p-1", warning)
	}

	stored, err := store.Get(ctx, "p-2")
	if err != nil {
		t.Fatalf("the warned partner was not stored: %v", err)
	}
	if stored.TaxID != "0123456789" {
		t.Errorf("TaxID = %q, want it stored normalised", stored.TaxID)
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Errorf("partners = %d, want 2", len(listed))
	}
}

// Re-saving a partner under the values it already has is an update that changes
// nothing — and it must not look like a partner that does not exist. MySQL
// reports the rows an UPDATE changed, not the rows it matched, so an existence
// check built on that count refuses an update that did exactly what it was asked
// to.
func TestStore_AcceptsAnUpdateThatChangesNothing(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	p, err := partner.NewPartner("p-1", "Công ty Bình Minh", partner.TypeBoth, "0123456789", 30)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}
	if _, err := store.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	stored, err := store.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	warning, err := store.Update(ctx, stored)
	if err != nil {
		t.Fatalf("re-saving a partner unchanged = %v, want it to succeed", err)
	}
	if warning != nil {
		t.Errorf("re-saving a partner under its own tax id warns about %v", warning)
	}

	again, err := store.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if again.Name != stored.Name || again.TaxID != stored.TaxID || again.PaymentTerms != stored.PaymentTerms {
		t.Errorf("after update = %+v, want it unchanged", again)
	}
}

// The interface the MariaDB adapter is written against.
var _ partner.Repository = (*partnerstore.Store)(nil)

// An update that matched nothing is reported rather than returned as a success.
func TestStore_ReportsAnUnknownPartner(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	stranger, err := partner.NewPartner("p-nope", "Không có", partner.TypeCustomer, "", 0)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	if _, err := store.Get(ctx, "p-nope"); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Errorf("Get = %v, want ErrUnknownPartner", err)
	}
	if _, err := store.Update(ctx, stranger); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Errorf("Update = %v, want ErrUnknownPartner", err)
	}
	if err := store.Delete(ctx, "p-nope"); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Errorf("Delete = %v, want ErrUnknownPartner", err)
	}
}

// A partner that could not be stored at all never reaches the table.
func TestStore_RefusesAPartnerItCannotValidate(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	_, err := store.Create(ctx, partner.Partner{ID: "p-1", Name: "", Type: partner.TypeCustomer})
	if !errors.Is(err, partner.ErrInvalidPartner) {
		t.Fatalf("Create = %v, want ErrInvalidPartner", err)
	}
}
