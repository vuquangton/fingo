package idempotency_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fingo/internal/idempotency"
)

// countingHandler is a write endpoint that records how many times it actually
// ran, and answers with which run it was. The run number in the body is what
// makes a replayed answer distinguishable from a fresh one.
type countingHandler struct {
	mu     sync.Mutex
	calls  int
	status int
	header http.Header
	seen   []string // the bodies the handler was handed
}

func (h *countingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.calls++
	call := h.calls
	status := h.status
	h.mu.Unlock()

	body, _ := io.ReadAll(r.Body)
	h.mu.Lock()
	h.seen = append(h.seen, string(body))
	h.mu.Unlock()

	for name, values := range h.header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	if status >= 200 && status < 300 {
		fmt.Fprintf(w, "created#%d", call)
		return
	}
	fmt.Fprintf(w, "failed#%d", call)
}

func (h *countingHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.calls
}

func (h *countingHandler) bodies() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]string(nil), h.seen...)
}

func (h *countingHandler) setStatus(status int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.status = status
}

// call sends a request through a handler, with the key when there is one.
func call(handler http.Handler, method, target, key, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if key != "" {
		request.Header.Set(idempotency.Header, key)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// The whole point: the same request twice does the work once, and the second
// client is given the first client's answer.
func TestMiddleware_SameKeyAndSameRequestIsAnsweredFromTheRecord(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	first := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", first.Code)
	}
	if first.Body.String() != "created#1" {
		t.Fatalf("first response = %q, want created#1", first.Body.String())
	}

	second := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if second.Code != http.StatusOK {
		t.Fatalf("the retry = %d, want the recorded 200", second.Code)
	}
	if second.Body.String() != first.Body.String() {
		t.Errorf("the retry was answered %q, and the first request was answered %q, want the same",
			second.Body.String(), first.Body.String())
	}
	if got := handler.count(); got != 1 {
		t.Errorf("the handler ran %d times, want once: the retry reached it", got)
	}
}

// One key spent on one request. A different request under the same key is
// neither replayed nor performed.
func TestMiddleware_SameKeyWithADifferentRequestIsAConflict(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	if got := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`); got.Code != http.StatusOK {
		t.Fatalf("first request = %d, want 200", got.Code)
	}

	conflict := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":999}`)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("a different body under the same key = %d, want 409", conflict.Code)
	}
	if got := handler.count(); got != 1 {
		t.Errorf("the handler ran %d times, want once: a conflict must not be performed", got)
	}

	// The same key with a different target is a different request too.
	if got := call(wrapped, http.MethodPost, "/bills", "key-1", `{"total":100}`); got.Code != http.StatusConflict {
		t.Errorf("a different path under the same key = %d, want 409", got.Code)
	}
	if got := handler.count(); got != 1 {
		t.Errorf("the handler ran %d times, want once", got)
	}
}

// A different key is a different request, whatever it says.
func TestMiddleware_DifferentKeysAreProcessedSeparately(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	first := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	second := call(wrapped, http.MethodPost, "/invoices", "key-2", `{"total":100}`)

	if first.Body.String() != "created#1" || second.Body.String() != "created#2" {
		t.Errorf("responses = %q and %q, want both requests performed", first.Body.String(), second.Body.String())
	}
	if got := handler.count(); got != 2 {
		t.Errorf("the handler ran %d times, want 2", got)
	}
}

// Past its TTL a key is a new key, and the retry is performed.
func TestMiddleware_AnExpiredKeyIsTreatedAsNew(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(
		idempotency.NewMemoryStore(),
		idempotency.WithTTL(40*time.Millisecond),
	).Wrap(handler)

	if got := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`); got.Body.String() != "created#1" {
		t.Fatalf("first request = %q, want created#1", got.Body.String())
	}

	time.Sleep(80 * time.Millisecond)

	late := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if late.Body.String() != "created#2" {
		t.Errorf("a request after the TTL = %q, want it performed again as created#2", late.Body.String())
	}
	if late.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", late.Code)
	}
	if got := handler.count(); got != 2 {
		t.Errorf("the handler ran %d times, want 2: the record had expired", got)
	}
}

// The key is optional, and without one nothing is recorded or replayed.
func TestMiddleware_WithoutAKeyTheRequestIsProcessedEveryTime(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	for i := 0; i < 3; i++ {
		got := call(wrapped, http.MethodPost, "/invoices", "", `{"total":100}`)
		if got.Code != http.StatusOK {
			t.Fatalf("request %d = %d, want 200", i+1, got.Code)
		}
	}
	if got := handler.count(); got != 3 {
		t.Errorf("the handler ran %d times, want 3", got)
	}
}

// Only writes are guarded. A read carries no key, and a read that carried one by
// mistake is still a read.
func TestMiddleware_ReadsAreNotRecorded(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		for i := 0; i < 2; i++ {
			call(wrapped, method, "/invoices", "key-1", "")
		}
	}

	if got := handler.count(); got != 6 {
		t.Errorf("the handler ran %d times, want 6: reads are not deduplicated", got)
	}
}

// What is replayed is the whole answer: the status the handler chose, and the
// headers it set.
func TestMiddleware_ReplaysTheStatusAndHeaders(t *testing.T) {
	handler := &countingHandler{
		status: http.StatusCreated,
		header: http.Header{"Location": []string{"/invoices/inv-1"}},
	}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	first := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if first.Code != http.StatusCreated || first.Header().Get("Location") != "/invoices/inv-1" {
		t.Fatalf("first request = %d %q, want 201 with a Location", first.Code, first.Header().Get("Location"))
	}

	second := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if second.Code != http.StatusCreated {
		t.Errorf("the replay = %d, want the recorded 201", second.Code)
	}
	if got := second.Header().Get("Location"); got != "/invoices/inv-1" {
		t.Errorf("the replay's Location = %q, want it replayed", got)
	}
	if second.Body.String() != first.Body.String() {
		t.Errorf("the replay body = %q, want %q", second.Body.String(), first.Body.String())
	}
}

// A failure is not an answer. Recording one would make a transient fault
// permanent for the life of the key, and a retry is exactly what a client should
// do about it.
func TestMiddleware_DoesNotRecordAFailure(t *testing.T) {
	handler := &countingHandler{status: http.StatusInternalServerError}
	wrapped := idempotency.New(idempotency.NewMemoryStore(), idempotency.WithTTL(time.Hour)).Wrap(handler)

	if got := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`); got.Code != http.StatusInternalServerError {
		t.Fatalf("first request = %d, want 500", got.Code)
	}

	// The fault passes, and the client retries under the same key.
	handler.setStatus(http.StatusOK)

	retry := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if retry.Code != http.StatusOK {
		t.Fatalf("the retry = %d, want it performed, not replayed as a 500", retry.Code)
	}
	if retry.Body.String() != "created#2" {
		t.Errorf("the retry = %q, want created#2", retry.Body.String())
	}

	// And now that it succeeded, it is recorded.
	again := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if again.Body.String() != "created#2" {
		t.Errorf("after a success the retry was answered %q, want the recorded created#2", again.Body.String())
	}
	if got := handler.count(); got != 2 {
		t.Errorf("the handler ran %d times, want 2", got)
	}
}

// The wrapper must not eat the request it wraps. A handler that has to read the
// body still reads all of it.
func TestMiddleware_HandsTheHandlerTheWholeRequest(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	body := `{"total":100,"note":"giao hàng"}`
	call(wrapped, http.MethodPost, "/invoices", "key-1", body)

	seen := handler.bodies()
	if len(seen) != 1 || seen[0] != body {
		t.Fatalf("the handler was handed %q, want %q", seen, body)
	}
}

// A retry carries new headers — an Authorization token, a Date, a trace id — and
// is still the same request. The hash covers what was asked for, not how it was
// dressed.
func TestMiddleware_ARetryWithDifferentHeadersIsStillTheSameRequest(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	first := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(`{"total":100}`))
	first.Header.Set(idempotency.Header, "key-1")
	first.Header.Set("Authorization", "Bearer first-token")

	recorder := httptest.NewRecorder()
	wrapped.ServeHTTP(recorder, first)

	retry := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(`{"total":100}`))
	retry.Header.Set(idempotency.Header, "key-1")
	retry.Header.Set("Authorization", "Bearer second-token")
	retry.Header.Set("X-Request-Id", "trace-2")

	replayed := httptest.NewRecorder()
	wrapped.ServeHTTP(replayed, retry)

	if replayed.Code != http.StatusOK {
		t.Fatalf("the retry = %d, want it replayed as 200", replayed.Code)
	}
	if replayed.Body.String() != recorder.Body.String() {
		t.Errorf("the retry = %q, want the first answer %q", replayed.Body.String(), recorder.Body.String())
	}
	if got := handler.count(); got != 1 {
		t.Errorf("the handler ran %d times, want once", got)
	}
}

// Two arrivals at the same moment are one request. Both get the same answer, and
// the handler runs once — the guarantee is worth nothing if it only holds when
// the retry is slow.
func TestMiddleware_ConcurrentArrivalsRunTheHandlerOnce(t *testing.T) {
	handler := &countingHandler{}
	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	const arrivals = 8

	bodies := make([]string, arrivals)
	codes := make([]int, arrivals)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < arrivals; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			got := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
			bodies[i] = got.Body.String()
			codes[i] = got.Code
		}(i)
	}

	close(start)
	wg.Wait()

	if got := handler.count(); got != 1 {
		t.Errorf("the handler ran %d times, want once", got)
	}

	for i := 0; i < arrivals; i++ {
		if codes[i] != http.StatusOK {
			t.Errorf("arrival %d = %d, want 200", i, codes[i])
		}
		if bodies[i] != "created#1" {
			t.Errorf("arrival %d was answered %q, want the one answer created#1", i, bodies[i])
		}
	}
}

// A store that cannot be read means it is unknown whether this request has
// already happened. Running it would risk the duplicate the middleware exists to
// prevent, so it is refused instead.
func TestMiddleware_RefusesTheRequestWhenTheStoreCannotBeRead(t *testing.T) {
	handler := &countingHandler{}
	store := &brokenStore{getErr: errors.New("the disk went away")}
	wrapped := idempotency.New(store).Wrap(handler)

	got := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if got.Code != http.StatusServiceUnavailable {
		t.Fatalf("a request that cannot be checked = %d, want 503", got.Code)
	}
	if handler.count() != 0 {
		t.Errorf("the handler ran %d times, want none: it must not run unchecked", handler.count())
	}
}

// A store that fails to record an answer does not undo the work the handler did.
// The client is owed its success; the missed record is reported to the operator.
func TestMiddleware_ReportsAnAnswerItCouldNotRecord(t *testing.T) {
	handler := &countingHandler{}
	store := &brokenStore{putErr: errors.New("the disk is full")}

	var reported []error
	wrapped := idempotency.New(store, idempotency.WithErrorHandler(func(err error) {
		reported = append(reported, err)
	})).Wrap(handler)

	got := call(wrapped, http.MethodPost, "/invoices", "key-1", `{"total":100}`)
	if got.Code != http.StatusOK {
		t.Fatalf("the request = %d, want the handler's 200: the work was done", got.Code)
	}
	if got.Body.String() != "created#1" {
		t.Errorf("body = %q, want created#1", got.Body.String())
	}
	if len(reported) != 1 {
		t.Fatalf("reported %d errors, want 1: losing the record is worth knowing about", len(reported))
	}
}

// Only arrivals of the SAME key wait for each other. Two different writes run at
// the same time, and this proves it: every handler has to be inside at once
// before any of them may finish, which cannot happen if the middleware holds one
// lock across the handler it wraps.
func TestMiddleware_DoesNotSerialiseRequestsWithDifferentKeys(t *testing.T) {
	const arrivals = 4

	var mu sync.Mutex
	inside := 0
	everyoneIn := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inside++
		if inside == arrivals {
			close(everyoneIn)
		}
		mu.Unlock()

		select {
		case <-everyoneIn:
		case <-time.After(2 * time.Second):
			t.Error("this request ran alone: the middleware is serialising different keys")
		}

		_, _ = io.WriteString(w, "created")
	})

	wrapped := idempotency.New(idempotency.NewMemoryStore()).Wrap(handler)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < arrivals; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			call(wrapped, http.MethodPost, "/invoices", fmt.Sprintf("key-%d", i), `{"total":100}`)
		}(i)
	}

	close(start)
	wg.Wait()

	mu.Lock()
	ran := inside
	mu.Unlock()

	if ran != arrivals {
		t.Errorf("%d handlers ran, want %d", ran, arrivals)
	}
}

// brokenStore fails whichever half of the store a test wants to break, and
// behaves like an empty store otherwise.
type brokenStore struct {
	getErr error
	putErr error
}

func (s *brokenStore) Get(context.Context, string) (idempotency.Record, bool, error) {
	if s.getErr != nil {
		return idempotency.Record{}, false, s.getErr
	}

	return idempotency.Record{}, false, nil
}

func (s *brokenStore) Put(context.Context, idempotency.Record) error { return s.putErr }

func (s *brokenStore) DeleteExpired(context.Context, time.Time) (int, error) { return 0, nil }
