package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Middleware answers a repeated write from the answer it already gave.
//
// It is a wrapper, not a participant: the handler underneath is handed the
// request it would have been handed anyway, and has no way to tell whether this
// is the first arrival or the fifth.
type Middleware struct {
	store Store
	ttl   time.Duration

	// onError is told about a failure that could not be reported to the client
	// because the work had already been done. It is nil unless one was set.
	onError func(error)

	mu       sync.Mutex
	inFlight map[string]*flight
}

// flight is one request that is currently running, and the waiters behind it.
type flight struct {
	done chan struct{}
}

// Option configures a Middleware.
type Option func(*Middleware)

// WithTTL changes how long an answer is kept. The default is TTL.
func WithTTL(ttl time.Duration) Option {
	return func(m *Middleware) { m.ttl = ttl }
}

// WithErrorHandler is told about errors that cannot be returned to the client.
//
// There is one: the store failing to record an answer after the handler has
// already done the work. The client is owed its success — the write happened —
// so the failure cannot be turned into a 5xx, but it means a later retry will
// not be recognised. That is worth logging, and this is where it goes.
func WithErrorHandler(report func(error)) Option {
	return func(m *Middleware) { m.onError = report }
}

// New builds the middleware over a store.
func New(store Store, opts ...Option) *Middleware {
	m := &Middleware{
		store:    store,
		ttl:      TTL,
		inFlight: make(map[string]*flight),
	}

	for _, opt := range opts {
		opt(m)
	}
	if m.ttl <= 0 {
		m.ttl = TTL
	}

	return m
}

// Wrap returns a handler that gives a repeated request the answer it already
// had.
//
// A request is a repeat when it carries an Idempotency-Key this middleware has
// seen, and the same request when it also hashes the same. One key, one request;
// one key, a different request is a 409.
//
// The whole of the response is buffered before it is sent, because an answer
// that is to be recorded has to be in hand before the client is told it
// succeeded. That is the right trade for the JSON-sized replies of a write API
// and the wrong one for a streamed download, which is why only mutating methods
// pass through here at all.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get(Header))
		if key == "" || !mutating(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		body, err := readBody(r)
		if err != nil {
			http.Error(w, "the request body could not be read", http.StatusBadRequest)
			return
		}

		response, err := m.resolve(r.Context(), next, r, key, requestHash(r, body))
		if err != nil {
			if errors.Is(err, ErrKeyReused) {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			// The store could not be read, so whether this request has already
			// been answered is unknown. Running it would risk exactly the
			// duplicate this middleware exists to prevent, so it is refused
			// instead: a client that retries loses nothing.
			http.Error(w, "the request could not be checked for a repeat; try again", http.StatusServiceUnavailable)
			return
		}

		writeResponse(w, response)
	})
}

// resolve returns the response this request is owed.
//
// The loop is what makes two arrivals of one request resolve to one answer.
// Whichever request gets to be the leader runs the handler; the rest wait for
// it, then find its answer in the record. A leader that recorded nothing — it
// failed, and a failure is not an answer — leaves the next waiter to try.
//
// Becoming the leader is a claim, and a claim is checked twice. A request that
// looked for a record, then found none, then claimed the slot cannot know that
// nothing was recorded in between: the request that was running may have
// finished and recorded its answer while this one was reading. So the record is
// looked for once more after the claim, and an answer found there is replayed
// instead of being performed a second time.
func (m *Middleware) resolve(ctx context.Context, next http.Handler, r *http.Request, key, hash string) (Response, error) {
	for {
		if response, answered, err := m.recorded(ctx, key, hash); answered || err != nil {
			return response, err
		}

		m.mu.Lock()
		running, busy := m.inFlight[key]
		if !busy {
			running = &flight{done: make(chan struct{})}
			m.inFlight[key] = running
			m.mu.Unlock()

			if response, answered, err := m.recorded(ctx, key, hash); answered || err != nil {
				m.finish(key, running)

				return response, err
			}

			return m.execute(ctx, next, r, key, hash, running)
		}
		m.mu.Unlock()

		// Another arrival of this key is already running. Wait for it, then
		// look again: its answer is ours if it recorded one.
		<-running.done
	}
}

// recorded returns the answer already on file for a request, and whether there
// was one.
func (m *Middleware) recorded(ctx context.Context, key, hash string) (Response, bool, error) {
	record, found, err := m.lookup(ctx, key)
	if err != nil {
		return Response{}, false, err
	}
	if !found {
		return Response{}, false, nil
	}
	if record.RequestHash != hash {
		return Response{}, false, fmt.Errorf("%w: key %q", ErrKeyReused, key)
	}

	return record.Response, true, nil
}

// finish releases the claim on a key and wakes whoever is waiting on it.
func (m *Middleware) finish(key string, running *flight) {
	m.mu.Lock()
	delete(m.inFlight, key)
	m.mu.Unlock()

	close(running.done)
}

// execute runs the handler and records what it answered.
func (m *Middleware) execute(ctx context.Context, next http.Handler, r *http.Request, key, hash string, running *flight) (Response, error) {
	defer m.finish(key, running)

	capture := newCapture()
	next.ServeHTTP(capture, r)

	response := capture.response()

	// Only a success is an answer. A request that failed has not been done, and
	// its client must be able to retry it under the same key and have it
	// actually run — which is the point of retrying. Recording a 500 would make
	// one transient failure permanent for the life of the key.
	if response.Status < http.StatusOK || response.Status >= http.StatusMultipleChoices {
		return response, nil
	}

	err := m.store.Put(ctx, Record{
		Key:         key,
		RequestHash: hash,
		Response:    response,
		CreatedAt:   time.Now(),
	})
	if err != nil {
		// The work is done, so the client is owed its success. Reporting a
		// failure here would invite a retry that writes a second time.
		if m.onError != nil {
			m.onError(fmt.Errorf("record the answer to key %q: %w", key, err))
		}
	}

	return response, nil
}

// lookup returns a record that is still worth answering from.
//
// An expired record is absent, whether or not anything has swept it yet: the
// TTL is a rule about the answer, and a store that still holds the bytes does
// not change what they mean.
func (m *Middleware) lookup(ctx context.Context, key string) (Record, bool, error) {
	record, found, err := m.store.Get(ctx, key)
	if err != nil {
		return Record{}, false, fmt.Errorf("look up key %q: %w", key, err)
	}
	if !found || !m.fresh(record) {
		return Record{}, false, nil
	}

	return record, true, nil
}

// fresh reports whether a record is still within its TTL.
func (m *Middleware) fresh(record Record) bool {
	return time.Since(record.CreatedAt) < m.ttl
}

// StartReaper cleans expired records up in the background, on the middleware's
// own store and TTL.
func (m *Middleware) StartReaper(every time.Duration) *Reaper {
	return NewReaper(m.store, m.ttl, every).Start()
}

// mutating reports whether a method changes anything. Only these are guarded:
// a GET repeated is a GET repeated, and recording every read would grow the
// store with answers nobody needs twice.
func mutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}

	return false
}

// readBody reads the request body and puts it back.
//
// It is read rather than streamed because the hash has to cover it, and it is
// put back so the handler reads the same bytes. A body is read once, so a
// wrapper that consumed it without restoring it would hand the handler an empty
// request.
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()

	r.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}

// requestHash identifies a request by what it asks for.
//
// Method, path, query and body — the parts a client writes and would send again
// unchanged. Headers are left out on purpose: a retry carries a new Date, a new
// Authorization token, a new trace id, and a hash that moved with them would
// call every retry a conflict.
func requestHash(r *http.Request, body []byte) string {
	sum := sha256.New()

	sum.Write([]byte(r.Method))
	sum.Write([]byte{0})
	sum.Write([]byte(r.URL.RequestURI()))
	sum.Write([]byte{0})
	sum.Write(body)

	return hex.EncodeToString(sum.Sum(nil))
}

// capture is a ResponseWriter that keeps the response instead of sending it.
type capture struct {
	header http.Header
	status int
	body   bytes.Buffer
	wrote  bool
}

func newCapture() *capture { return &capture{header: make(http.Header)} }

func (c *capture) Header() http.Header { return c.header }

func (c *capture) WriteHeader(status int) {
	if c.wrote {
		return
	}
	c.status = status
	c.wrote = true
}

func (c *capture) Write(p []byte) (int, error) {
	if !c.wrote {
		c.WriteHeader(http.StatusOK)
	}

	return c.body.Write(p)
}

// response is what the handler wrote, as a value that can be stored.
func (c *capture) response() Response {
	status := c.status
	if !c.wrote {
		status = http.StatusOK
	}

	header := make(http.Header, len(c.header))
	for name, values := range c.header {
		// Recomputed from the body when it is sent, and a length left over from
		// another answer is a truncated reply.
		if strings.EqualFold(name, "Content-Length") {
			continue
		}
		header[name] = append([]string(nil), values...)
	}

	return Response{
		Status: status,
		Header: header,
		Body:   append([]byte(nil), c.body.Bytes()...),
	}
}

// writeResponse sends a response, recorded or fresh.
func writeResponse(w http.ResponseWriter, response Response) {
	for name, values := range response.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	status := response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)

	if len(response.Body) > 0 {
		_, _ = w.Write(response.Body)
	}
}
