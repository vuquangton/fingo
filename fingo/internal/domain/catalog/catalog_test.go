package catalog_test

import (
	"testing"

	"fingo/internal/domain/catalog"
)

func TestCustomer_StubInvariants(t *testing.T) {
	c := catalog.NewCustomerStub("c-100", "KH001", "Acme Vietnam", "0109998888")
	if c.Code != "KH001" || !c.IsActive {
		t.Fatalf("unexpected customer stub: %+v", c)
	}
}

func TestItem_StubInvariants(t *testing.T) {
	item := catalog.NewItemStub("i-100", "VT001", "Office Paper A4", "Ream")
	if item.ItemType != catalog.ItemTypeGood || item.BaseUOM != "Ream" {
		t.Fatalf("unexpected item stub: %+v", item)
	}
}
