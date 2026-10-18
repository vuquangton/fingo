package idempotency

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// MemoryStore keeps records in memory.
//
// It is enough to run against and to build a screen on, and it is what the tests
// use. It is not enough for a deployment: a desktop client that restarts has
// forgotten every answer it gave, so a retry that crosses a restart is processed
// again. A durable store belongs behind the same interface — the schema is the
// one in migration 00007's shape, a key, a hash, an answer and a timestamp.
//
// The zero value is an empty store.
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]Record
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// Get returns the record for a key, and whether there was one.
//
// The record is a copy, headers and body included. A caller that edited what it
// was handed would otherwise be editing the answer that a later retry is given.
func (s *MemoryStore) Get(_ context.Context, key string) (Record, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[key]
	if !ok {
		return Record{}, false, nil
	}

	return cloneRecord(record), true, nil
}

// Put stores a record, replacing any record already under the key.
//
// Replacing is deliberate: the only time a Put follows an existing record is
// when the record had expired, and the new answer is the one that now stands.
func (s *MemoryStore) Put(_ context.Context, record Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.records == nil {
		s.records = make(map[string]Record)
	}
	s.records[record.Key] = cloneRecord(record)

	return nil
}

// DeleteExpired removes every record created before the cutoff, and reports how
// many went.
func (s *MemoryStore) DeleteExpired(_ context.Context, before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for key, record := range s.records {
		if record.CreatedAt.Before(before) {
			delete(s.records, key)
			removed++
		}
	}

	return removed, nil
}

// cloneRecord copies a record deeply enough that no caller shares a header map
// or a body slice with the store.
func cloneRecord(record Record) Record {
	cloned := record

	if record.Response.Header != nil {
		cloned.Response.Header = make(http.Header, len(record.Response.Header))
		for name, values := range record.Response.Header {
			cloned.Response.Header[name] = append([]string(nil), values...)
		}
	}
	if record.Response.Body != nil {
		cloned.Response.Body = append([]byte(nil), record.Response.Body...)
	}

	return cloned
}
