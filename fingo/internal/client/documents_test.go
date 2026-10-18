package client_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"fingo/internal/client"
)

// doc builds a document with no client on it, which is how a caller that has
// nothing to hide writes one.
func doc(id, number string, date time.Time, total int64) client.Document {
	return client.Document{ID: id, Number: number, Date: date, Total: total}
}

// The test the objective names: two clients, and neither can see the other's
// documents — not by listing, not by asking for a known id, not by deleting.
func TestDocumentRepository_KeepsTwoClientsDocumentsApart(t *testing.T) {
	ctx := context.Background()
	repo := client.NewMemoryDocumentRepository()

	a := mustScope(t, "c-a")
	b := mustScope(t, "c-b")

	// The same document id in both clients is not a clash: a document id is
	// unique within a client's books, not across the practice.
	for _, put := range []struct {
		scope client.Scope
		d     client.Document
	}{
		{a, doc("d-1", "HD-1", day1, 100)},
		{b, doc("d-1", "HD-B1", day3, 300)},
		{b, doc("d-2", "HD-B2", day2, 200)},
	} {
		if err := repo.Put(ctx, put.scope, put.d); err != nil {
			t.Fatalf("Put(%s, %s): %v", put.scope.ClientID(), put.d.ID, err)
		}
	}

	listA, err := repo.List(ctx, a)
	if err != nil {
		t.Fatalf("List(c-a): %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("client A lists %d documents, want 1: %+v", len(listA), listA)
	}
	if listA[0].ID != "d-1" || listA[0].Number != "HD-1" || listA[0].Total != 100 {
		t.Errorf("client A's document = %+v, want HD-1 for 100", listA[0])
	}
	if listA[0].ClientID != "c-a" {
		t.Errorf("client A's document is stamped %q, want c-a", listA[0].ClientID)
	}

	listB, err := repo.List(ctx, b)
	if err != nil {
		t.Fatalf("List(c-b): %v", err)
	}
	if len(listB) != 2 {
		t.Fatalf("client B lists %d documents, want 2: %+v", len(listB), listB)
	}
	// Oldest first, and both of B's, never A's.
	if listB[0].Number != "HD-B2" || listB[1].Number != "HD-B1" {
		t.Errorf("client B's documents = %s, %s; want HD-B2 then HD-B1", listB[0].Number, listB[1].Number)
	}

	// One id, two clients, two different documents.
	for scope, want := range map[client.Scope]string{a: "HD-1", b: "HD-B1"} {
		got, err := repo.Get(ctx, scope, "d-1")
		if err != nil {
			t.Fatalf("Get(%s, d-1): %v", scope.ClientID(), err)
		}
		if got.Number != want {
			t.Errorf("Get(%s, d-1).Number = %q, want %q", scope.ClientID(), got.Number, want)
		}
	}

	// Client B's document is not client A's, whatever A asks for.
	if _, err := repo.Get(ctx, a, "d-2"); !errors.Is(err, client.ErrUnknownDocument) {
		t.Errorf("client A looked up client B's d-2 and got %v, want ErrUnknownDocument", err)
	}

	// And A cannot delete it.
	if err := repo.Delete(ctx, a, "d-2"); !errors.Is(err, client.ErrUnknownDocument) {
		t.Errorf("client A deleted client B's d-2: %v", err)
	}
	if _, err := repo.Get(ctx, b, "d-2"); err != nil {
		t.Errorf("client B's d-2 did not survive client A's delete: %v", err)
	}
}

// A lookup for another client's document must be indistinguishable from a lookup
// for one that does not exist. Reporting "that is somebody else's" would let a
// caller enumerate another client's records one id at a time.
func TestDocumentRepository_AnotherClientsDocumentLooksUnknown(t *testing.T) {
	ctx := context.Background()
	repo := client.NewMemoryDocumentRepository()

	if err := repo.Put(ctx, mustScope(t, "c-a"), doc("d-1", "HD-1", day1, 100)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	_, err := repo.Get(ctx, mustScope(t, "c-b"), "d-1")
	if !errors.Is(err, client.ErrUnknownDocument) {
		t.Fatalf("Get = %v, want ErrUnknownDocument", err)
	}
	if errors.Is(err, client.ErrForeignClient) {
		t.Fatal("the error says the document exists and belongs to another client, which is a leak")
	}

	// Same for a document that genuinely does not exist anywhere.
	_, err = repo.Get(ctx, mustScope(t, "c-b"), "d-nope")
	if !errors.Is(err, client.ErrUnknownDocument) {
		t.Fatalf("Get of a missing document = %v, want ErrUnknownDocument", err)
	}
}

// "List every document" is not an operation this repository can express. Every
// method refuses a scope that names nobody, before it touches a row.
func TestDocumentRepository_RefusesEveryUnscopedQuery(t *testing.T) {
	ctx := context.Background()
	repo := client.NewMemoryDocumentRepository()

	// A row exists, so a repository missing its guard would answer these with
	// data rather than with an error.
	if err := repo.Put(ctx, mustScope(t, "c-a"), doc("d-1", "HD-1", day1, 100)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	var zero client.Scope

	calls := map[string]func() error{
		"Put": func() error {
			return repo.Put(ctx, zero, doc("d-9", "HD-9", day1, 900))
		},
		"Get": func() error {
			_, err := repo.Get(ctx, zero, "d-1")

			return err
		},
		"List": func() error {
			_, err := repo.List(ctx, zero)

			return err
		},
		"Delete": func() error {
			return repo.Delete(ctx, zero, "d-1")
		},
	}

	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, client.ErrNoScope) {
				t.Fatalf("%s with no scope = %v, want ErrNoScope", name, err)
			}
		})
	}

	// Nothing leaked and nothing was written: the one row is still A's alone.
	list, err := repo.List(ctx, mustScope(t, "c-a"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != "d-1" {
		t.Fatalf("the unscoped calls changed the book: %+v", list)
	}
}

// A write that names a client other than the one the request was authenticated
// for is refused, not quietly refiled. The scope wins; it is not a default the
// caller may override.
func TestDocumentRepository_RefusesAWriteThatNamesAnotherClient(t *testing.T) {
	ctx := context.Background()
	repo := client.NewMemoryDocumentRepository()

	d := doc("d-1", "HD-1", day1, 100)
	d.ClientID = "c-b"

	err := repo.Put(ctx, mustScope(t, "c-a"), d)
	if !errors.Is(err, client.ErrForeignClient) {
		t.Fatalf("Put = %v, want ErrForeignClient", err)
	}

	// The document was not written under either client.
	for _, id := range []string{"c-a", "c-b"} {
		list, err := repo.List(ctx, mustScope(t, id))
		if err != nil {
			t.Fatalf("List(%s): %v", id, err)
		}
		if len(list) != 0 {
			t.Errorf("client %s has %+v, want nothing", id, list)
		}
	}
}

// The client column is written from the scope, so a row cannot end up filed
// under a client the request did not name — including by omission.
func TestDocumentRepository_StampsTheClientFromTheScope(t *testing.T) {
	ctx := context.Background()
	repo := client.NewMemoryDocumentRepository()

	if err := repo.Put(ctx, mustScope(t, "c-a"), doc("d-1", "HD-1", day1, 100)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := repo.Get(ctx, mustScope(t, "c-a"), "d-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ClientID != "c-a" {
		t.Errorf("client id = %q, want c-a", got.ClientID)
	}
}

// A document that cannot be stored is refused before it is filed, and a stored
// one is not silently overwritten.
func TestDocumentRepository_RefusesWhatCannotBeStored(t *testing.T) {
	ctx := context.Background()
	repo := client.NewMemoryDocumentRepository()
	scope := mustScope(t, "c-a")

	cases := map[string]client.Document{
		"no id":     {Number: "HD-1", Date: day1},
		"no number": {ID: "d-1", Date: day1},
		"no date":   {ID: "d-1", Number: "HD-1"},
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			if err := repo.Put(ctx, scope, d); !errors.Is(err, client.ErrInvalidDocument) {
				t.Fatalf("Put = %v, want ErrInvalidDocument", err)
			}
		})
	}

	if err := repo.Put(ctx, scope, doc("d-1", "HD-1", day1, 100)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := repo.Put(ctx, scope, doc("d-1", "HD-1-again", day1, 999)); !errors.Is(err, client.ErrDocumentExists) {
		t.Fatalf("re-writing a stored document = %v, want ErrDocumentExists", err)
	}
}
