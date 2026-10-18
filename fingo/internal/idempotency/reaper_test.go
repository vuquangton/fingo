package idempotency_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fingo/internal/idempotency"
)

func record(key, hash string, age time.Duration) idempotency.Record {
	return idempotency.Record{
		Key:         key,
		RequestHash: hash,
		Response:    idempotency.Response{Status: http.StatusOK, Body: []byte("created")},
		CreatedAt:   time.Now().Add(-age),
	}
}

func TestMemoryStore_PutsAndGets(t *testing.T) {
	ctx := context.Background()
	store := idempotency.NewMemoryStore()

	if _, found, err := store.Get(ctx, "key-1"); err != nil || found {
		t.Fatalf("an empty store found %v (err %v), want nothing", found, err)
	}

	if err := store.Put(ctx, record("key-1", "hash-1", 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, found, err := store.Get(ctx, "key-1")
	if err != nil || !found {
		t.Fatalf("Get: found %v, err %v", found, err)
	}
	if got.RequestHash != "hash-1" || got.Response.Status != http.StatusOK || string(got.Response.Body) != "created" {
		t.Errorf("record = %+v", got)
	}
}

// The store hands out copies. A caller that edited what it was given would
// otherwise be editing the answer a later retry is given.
func TestMemoryStore_HandsOutCopies(t *testing.T) {
	ctx := context.Background()
	store := idempotency.NewMemoryStore()

	stored := record("key-1", "hash-1", 0)
	stored.Response.Header = http.Header{"Location": []string{"/invoices/inv-1"}}
	stored.Response.Body = []byte("created")
	if err := store.Put(ctx, stored); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Editing what was put in afterwards does not reach the store.
	stored.Response.Body[0] = 'X'
	stored.Response.Header.Set("Location", "/tampered")

	got, _, err := store.Get(ctx, "key-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.Response.Body) != "created" {
		t.Errorf("body = %q, want the stored created", got.Response.Body)
	}
	if got.Response.Header.Get("Location") != "/invoices/inv-1" {
		t.Errorf("Location = %q, want the stored one", got.Response.Header.Get("Location"))
	}

	// Nor does editing what was got out.
	got.Response.Body[0] = 'Y'
	got.Response.Header.Set("Location", "/tampered-again")

	again, _, err := store.Get(ctx, "key-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(again.Response.Body) != "created" || again.Response.Header.Get("Location") != "/invoices/inv-1" {
		t.Errorf("the store changed under a caller: %+v", again.Response)
	}
}

func TestMemoryStore_DeleteExpiredKeepsWhatIsStillGood(t *testing.T) {
	ctx := context.Background()
	store := idempotency.NewMemoryStore()

	if err := store.Put(ctx, record("old", "hash", 25*time.Hour)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, record("fresh", "hash", time.Hour)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	removed, err := store.DeleteExpired(ctx, time.Now().Add(-idempotency.TTL))
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d, want the 1 expired record", removed)
	}

	if _, found, _ := store.Get(ctx, "old"); found {
		t.Error("the expired record is still there")
	}
	if _, found, _ := store.Get(ctx, "fresh"); !found {
		t.Error("the fresh record went with it")
	}
}

// The reaper's one pass, with records planted at known ages so the sweep does
// not depend on waiting.
func TestReaper_SweepRemovesExpiredRecords(t *testing.T) {
	ctx := context.Background()
	store := idempotency.NewMemoryStore()

	if err := store.Put(ctx, record("old", "hash", idempotency.TTL+time.Hour)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, record("fresh", "hash", time.Minute)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	reaper := idempotency.NewReaper(store, idempotency.TTL, time.Hour)

	removed, err := reaper.Sweep(ctx)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 1 {
		t.Errorf("swept %d, want the 1 expired record", removed)
	}

	if _, found, _ := store.Get(ctx, "old"); found {
		t.Error("the expired record survived the sweep")
	}
	if _, found, _ := store.Get(ctx, "fresh"); !found {
		t.Error("the fresh record was swept")
	}
}

// And in the background, which is the only way it runs in a deployment.
func TestReaper_SweepsInTheBackground(t *testing.T) {
	ctx := context.Background()
	store := idempotency.NewMemoryStore()

	if err := store.Put(ctx, record("old", "hash", idempotency.TTL+time.Hour)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	reaper := idempotency.NewReaper(store, idempotency.TTL, 5*time.Millisecond).Start()
	defer reaper.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, found, _ := store.Get(ctx, "old"); !found {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Error("the expired record was never swept in the background")
}

// Stopping is safe whatever state the reaper is in: never started, started
// twice, stopped twice.
func TestReaper_StartAndStopAreSafeToRepeat(t *testing.T) {
	store := idempotency.NewMemoryStore()
	reaper := idempotency.NewReaper(store, idempotency.TTL, time.Hour)

	reaper.Stop() // never started

	reaper.Start()
	reaper.Start() // already running
	reaper.Stop()
	reaper.Stop() // already stopped
}

// The middleware offers its own store and TTL to the reaper, so a caller does
// not have to keep the two in step by hand.
func TestMiddleware_StartsAReaperOverItsOwnStore(t *testing.T) {
	ctx := context.Background()
	store := idempotency.NewMemoryStore()
	middleware := idempotency.New(store, idempotency.WithTTL(time.Nanosecond))

	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("created"))
	}))

	// A key recorded now is expired a nanosecond later.
	if got := call(handler, http.MethodPost, "/invoices", "key-1", `{"total":100}`); got.Code != http.StatusOK {
		t.Fatalf("request = %d, want 200", got.Code)
	}

	reaper := middleware.StartReaper(5 * time.Millisecond)
	defer reaper.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, found, _ := store.Get(ctx, "key-1"); !found {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Error("the middleware's reaper never swept its store")
}

// A nil body is a body: a POST with nothing in it is still a POST.
func TestMiddleware_AcceptsARequestWithNoBody(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	first := httptest.NewRequest(http.MethodPost, "/invoices", nil)
	first.Header.Set(idempotency.Header, "key-1")
	firstRecorder := httptest.NewRecorder()
	wrapped.ServeHTTP(firstRecorder, first)

	second := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(""))
	second.Header.Set(idempotency.Header, "key-1")
	secondRecorder := httptest.NewRecorder()
	wrapped.ServeHTTP(secondRecorder, second)

	if got := handler.count(); got != 1 {
		t.Errorf("the handler ran %d times, want once", got)
	}
	if secondRecorder.Code != http.StatusOK {
		t.Errorf("the retry = %d, want 200", secondRecorder.Code)
	}
	if secondRecorder.Body.String() != firstRecorder.Body.String() {
		t.Errorf("the retry = %q, want the first answer %q",
			secondRecorder.Body.String(), firstRecorder.Body.String())
	}
}
