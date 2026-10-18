// Package idempotency makes a repeated write safe to retry.
//
// A client that does not know whether its request arrived sends it again. A
// double-clicked button, a retried socket, an operator repeating a step they
// were unsure about: all deliver the same write twice, and nothing downstream
// can tell the second from a real second one. This package answers the second
// arrival with the first answer, and does not let it reach the handler.
//
// It works in the transport, not in the business logic. The handler that does
// the work is wrapped and never learns that the wrapper exists: the key, the
// request hash and the recorded answer live here, and a use case that has to
// remember to be idempotent is a use case that will one day forget. (The ledger
// does carry its own idempotency key, recorded in the posting transaction —
// see migration 00007. That is a different guarantee, and the two are not
// alternatives: this one stops a request reaching the handler at all, and that
// one stops a posting reaching the ledger.)
//
// The contract, in full:
//
//   - A mutating request may carry an Idempotency-Key header. Without one, the
//     request is processed normally and nothing is recorded.
//   - The first request with a key runs the handler and records the answer.
//   - The same key with the same request is answered from the record, and the
//     handler is not run again.
//   - The same key with a different request is a 409 Conflict. It is not a
//     replay and not a fresh request, and answering it either way would be a lie
//     about what has already happened.
//   - A record is kept for TTL. After that the key is a new key, and a retry
//     that arrives that late runs the handler again.
package idempotency

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Header is the request header that names a request.
//
// The name is the one every HTTP client already knows; it is the header Stripe
// documents, and inventing another would only make callers look it up.
const Header = "Idempotency-Key"

// TTL is how long an answer is kept, and therefore the longest a client has to
// retry and still be recognised.
//
// A day is long enough for a retry that follows an overnight failure and short
// enough that the store does not grow without bound. It is not a retention
// policy for the records themselves — those are answers, not history; the
// documents and vouchers the handler wrote are the history.
const TTL = 24 * time.Hour

// ErrKeyReused is a key that has already been used for a different request.
//
// It reaches the client as 409 Conflict. The request is neither replayed nor
// performed: the key has been spent on something else, and the client has to
// send this one under a key of its own.
var ErrKeyReused = errors.New("idempotency: this key was already used for a different request")

// Response is the answer a request produced, kept so it can be given again.
type Response struct {
	// Status is the HTTP status the handler wrote.
	Status int

	// Header is what the handler set. Content-Length is not kept: it is
	// recomputed from the body, and a stale one is a truncated reply.
	Header http.Header

	// Body is the reply in full.
	Body []byte
}

// Record is one answered request.
type Record struct {
	// Key is the Idempotency-Key the request carried.
	Key string

	// RequestHash identifies the request. Two requests with one key and one
	// hash are the same request; the same key with a different hash is a
	// conflict.
	RequestHash string

	// Response is what was sent the first time.
	Response Response

	// CreatedAt is when the answer was recorded — the instant the TTL is
	// measured from.
	CreatedAt time.Time
}

// Store keeps records for as long as they are worth keeping.
//
// It is deliberately dumb: it does not know the TTL, and it does not decide what
// an expired record means. Expiry is the middleware's rule, in one place, so
// that a store can be swapped without the guarantee changing shape — and so
// that an expired record is treated as absent on read, whether or not anything
// has swept it yet.
type Store interface {
	// Get returns the record for a key, and whether there was one.
	Get(ctx context.Context, key string) (Record, bool, error)

	// Put stores a record, replacing any record already under the key.
	Put(ctx context.Context, record Record) error

	// DeleteExpired removes every record created before the cutoff, and reports
	// how many went.
	DeleteExpired(ctx context.Context, before time.Time) (int, error)
}
