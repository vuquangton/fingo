package partner_test

import (
	"context"
	"errors"
	"testing"

	"fingo/internal/partner"
)

// repo builds a directory with one partner already in it, and returns the
// directory, the partner that is stored, and the warning its creation produced.
func seeded(t *testing.T) (*partner.MemoryRepository, partner.Partner) {
	t.Helper()

	ctx := context.Background()
	dir := partner.NewMemoryRepository()

	p, err := partner.NewPartner("p-1", "Công ty Bình Minh", partner.TypeCustomer, "0123456789", 30)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	warning, err := dir.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if warning != nil {
		t.Fatalf("the first partner in an empty directory warns about %v", warning)
	}

	return dir, p
}

// The whole of the lifecycle: a partner goes in, comes back, changes, and goes
// away.
func TestMemoryRepository_CreatesReadsUpdatesAndDeletes(t *testing.T) {
	ctx := context.Background()
	dir, _ := seeded(t)

	got, err := dir.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Công ty Bình Minh" || got.Type != partner.TypeCustomer || got.PaymentTerms != 30 {
		t.Fatalf("round trip = %+v", got)
	}
	if !got.Active {
		t.Error("a stored partner is active")
	}

	listed, err := dir.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("partners = %d, want 1", len(listed))
	}

	// Update: the business is now also a supplier, on longer terms.
	changed := got
	changed.Name = "Công ty TNHH Bình Minh"
	changed.Type = partner.TypeBoth
	changed.PaymentTerms = 45

	warning, err := dir.Update(ctx, changed)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if warning != nil {
		t.Errorf("re-saving a partner under its own tax id warns about %v", warning)
	}

	again, err := dir.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if again.Name != "Công ty TNHH Bình Minh" || again.Type != partner.TypeBoth || again.PaymentTerms != 45 {
		t.Errorf("after update = %+v", again)
	}
	if !again.IsCustomer() || !again.IsSupplier() {
		t.Error("a partner who is both answers yes to both sides")
	}

	// Delete: gone, and gone for good.
	if err := dir.Delete(ctx, "p-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := dir.Get(ctx, "p-1"); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Fatalf("Get after delete = %v, want ErrUnknownPartner", err)
	}

	listed, err = dir.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("partners = %d after delete, want 0", len(listed))
	}
}

// A duplicate tax id is a warning, not a refusal: the second row is written and
// the operator is told, because whether the two rows are one business is a
// judgement the system cannot make on their behalf.
func TestMemoryRepository_WarnsAboutADuplicateTaxIDInsteadOfRefusingIt(t *testing.T) {
	ctx := context.Background()
	dir, _ := seeded(t)

	// The same business entered again — a branch, under a different name and
	// with the tax id written the way somebody would type it.
	second, err := partner.NewPartner("p-2", "Bình Minh — chi nhánh", partner.TypeSupplier, "0123 456 789", 15)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	warning, err := dir.Create(ctx, second)
	if err != nil {
		t.Fatalf("a duplicate tax id must not stop the create: %v", err)
	}
	if warning == nil {
		t.Fatal("a duplicate tax id must be reported")
	}
	if warning.Code != partner.WarningDuplicateTaxID {
		t.Errorf("Code = %q, want %q", warning.Code, partner.WarningDuplicateTaxID)
	}
	if warning.PartnerID != "p-1" {
		t.Errorf("PartnerID = %q, want the partner already holding the tax id", warning.PartnerID)
	}
	if warning.Message == "" {
		t.Error("a warning an operator cannot read is not a warning")
	}

	// Warned about, but written.
	stored, err := dir.Get(ctx, "p-2")
	if err != nil {
		t.Fatalf("the warned partner was not stored: %v", err)
	}
	if stored.TaxID != "0123456789" {
		t.Errorf("TaxID = %q, want it stored in the one form the directory compares on", stored.TaxID)
	}

	listed, err := dir.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Errorf("partners = %d, want 2: the warning is not a refusal", len(listed))
	}
}

// A warning is about a second holder of the tax id. Re-saving a partner under
// the tax id it already has is not one.
func TestMemoryRepository_WarnsOnlyWhenTheTaxIDBelongsToSomebodyElse(t *testing.T) {
	ctx := context.Background()
	dir, first := seeded(t)

	second, err := partner.NewPartner("p-2", "Nhà cung cấp Sông Đà", partner.TypeSupplier, "9876543210", 15)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}
	if warning, err := dir.Create(ctx, second); err != nil || warning != nil {
		t.Fatalf("a distinct tax id produced warning %v, error %v", warning, err)
	}

	// p-1 keeps its own tax id: nothing to warn about.
	if warning, err := dir.Update(ctx, first); err != nil || warning != nil {
		t.Fatalf("re-saving p-1 produced warning %v, error %v", warning, err)
	}

	// p-2 takes p-1's tax id: now there are two rows claiming one tax id.
	second.TaxID = "0123456789"
	warning, err := dir.Update(ctx, second)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if warning == nil || warning.PartnerID != "p-1" {
		t.Fatalf("warning = %v, want one naming p-1", warning)
	}
}

// Many partners may have no tax id. None of them duplicates another.
func TestMemoryRepository_SaysNothingAboutPartnersWithNoTaxID(t *testing.T) {
	ctx := context.Background()
	dir := partner.NewMemoryRepository()

	for _, id := range []string{"p-1", "p-2", "p-3"} {
		p, err := partner.NewPartner(id, "Khách lẻ "+id, partner.TypeCustomer, "", 0)
		if err != nil {
			t.Fatalf("NewPartner: %v", err)
		}

		warning, err := dir.Create(ctx, p)
		if err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
		if warning != nil {
			t.Errorf("%s: a partner with no tax id duplicates nothing, got %v", id, warning)
		}
	}
}

// One partner who is both is one record with two roles, which is the reason the
// type has a third value at all.
func TestMemoryRepository_KeepsOneRecordForAPartnerWhoIsBoth(t *testing.T) {
	ctx := context.Background()
	dir := partner.NewMemoryRepository()

	both, err := partner.NewPartner("p-1", "Công ty Bình Minh", partner.TypeBoth, "", 30)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}
	if _, err := dir.Create(ctx, both); err != nil {
		t.Fatalf("Create: %v", err)
	}

	listed, err := dir.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("partners = %d, want the single record that serves both sides", len(listed))
	}
	if !listed[0].IsCustomer() || !listed[0].IsSupplier() {
		t.Errorf("the stored partner is %q, which does not answer for both sides", listed[0].Type)
	}
}

// An id already in the directory is not overwritten: Create is a create, and an
// update is what changes a partner.
func TestMemoryRepository_RefusesToCreateTheSameIDTwice(t *testing.T) {
	ctx := context.Background()
	dir, _ := seeded(t)

	clash, err := partner.NewPartner("p-1", "Người khác", partner.TypeSupplier, "", 0)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	if _, err := dir.Create(ctx, clash); !errors.Is(err, partner.ErrPartnerExists) {
		t.Fatalf("Create of a taken id = %v, want ErrPartnerExists", err)
	}

	stored, err := dir.Get(ctx, "p-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Name != "Công ty Bình Minh" {
		t.Errorf("the stored partner changed under a refused create: %+v", stored)
	}
}

// Acting on a partner that is not there is reported, not silently ignored: an
// update that matched nothing must not look like a success.
func TestMemoryRepository_ReportsUnknownPartners(t *testing.T) {
	ctx := context.Background()
	dir, _ := seeded(t)

	stranger, err := partner.NewPartner("p-nope", "Không có", partner.TypeCustomer, "", 0)
	if err != nil {
		t.Fatalf("NewPartner: %v", err)
	}

	if _, err := dir.Get(ctx, "p-nope"); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Errorf("Get = %v, want ErrUnknownPartner", err)
	}
	if _, err := dir.Update(ctx, stranger); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Errorf("Update = %v, want ErrUnknownPartner", err)
	}
	if err := dir.Delete(ctx, "p-nope"); !errors.Is(err, partner.ErrUnknownPartner) {
		t.Errorf("Delete = %v, want ErrUnknownPartner", err)
	}
}

// A partner that could not be stored at all is refused before it reaches the
// directory.
func TestMemoryRepository_StoresNothingItCannotValidate(t *testing.T) {
	ctx := context.Background()
	dir := partner.NewMemoryRepository()

	_, err := dir.Create(ctx, partner.Partner{ID: "p-1", Name: "", Type: partner.TypeCustomer})
	if !errors.Is(err, partner.ErrInvalidPartner) {
		t.Fatalf("Create = %v, want ErrInvalidPartner", err)
	}

	listed, err := dir.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("partners = %d after a refused create, want 0", len(listed))
	}
}

// The rule itself, with no directory around it: which rows does a candidate
// duplicate?
func TestCheckDuplicateTaxID_NamesTheRowAlreadyHoldingTheTaxID(t *testing.T) {
	held := []partner.Partner{
		{ID: "p-1", Name: "Công ty Bình Minh", TaxID: "0123456789"},
		{ID: "p-2", Name: "Nhà cung cấp Sông Đà", TaxID: ""},
	}

	candidate := partner.Partner{ID: "p-new", Name: "Bình Minh (chi nhánh)", TaxID: "0123.456.789"}

	warning := partner.CheckDuplicateTaxID(candidate, held)
	if warning == nil {
		t.Fatal("the tax id is already held and must be reported")
	}
	if warning.PartnerID != "p-1" {
		t.Errorf("PartnerID = %q, want p-1", warning.PartnerID)
	}
	if warning.Code != partner.WarningDuplicateTaxID {
		t.Errorf("Code = %q, want %q", warning.Code, partner.WarningDuplicateTaxID)
	}
}

func TestCheckDuplicateTaxID_IsSilentWhenNothingIsDuplicated(t *testing.T) {
	held := []partner.Partner{{ID: "p-1", Name: "Công ty Bình Minh", TaxID: "0123456789"}}

	cases := map[string]partner.Partner{
		"no tax id at all":  {ID: "p-new", Name: "Khách lẻ"},
		"a distinct tax id": {ID: "p-new", Name: "Sông Đà", TaxID: "9876543210"},
		"the row itself":    {ID: "p-1", Name: "Công ty Bình Minh", TaxID: "0123456789"},
	}

	for name, candidate := range cases {
		t.Run(name, func(t *testing.T) {
			if warning := partner.CheckDuplicateTaxID(candidate, held); warning != nil {
				t.Errorf("warning = %v, want none", warning)
			}
		})
	}
}
