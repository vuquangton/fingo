package idempotency

import (
	"context"
	"sync"
	"time"
)

// Reaper deletes records that have expired, so the store does not grow by one
// row per write forever.
//
// It is not what makes the TTL work. An expired record is treated as absent the
// moment it is read — see Middleware.lookup — and the reaper only reclaims the
// space. A deployment that never starts one is correct and untidy; a deployment
// that relied on one would be wrong the first time the process was busy.
type Reaper struct {
	store Store
	ttl   time.Duration
	every time.Duration

	mu      sync.Mutex
	stop    chan struct{}
	stopped chan struct{}
	running bool
}

// NewReaper builds a reaper over a store.
//
// every is how often it sweeps. Sweeping hourly over a day-long TTL means a
// record is deleted at most an hour after it stopped being answerable, which is
// as much precision as a retention period needs.
func NewReaper(store Store, ttl, every time.Duration) *Reaper {
	if every <= 0 {
		every = time.Hour
	}

	return &Reaper{store: store, ttl: ttl, every: every}
}

// Sweep removes every expired record once, and reports how many went.
func (r *Reaper) Sweep(ctx context.Context) (int, error) {
	return r.store.DeleteExpired(ctx, time.Now().Add(-r.ttl))
}

// Start begins sweeping in the background, and returns the reaper so a caller
// can write StartReaper(...).Stop() if it wants nothing else.
func (r *Reaper) Start() *Reaper {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return r
	}

	r.stop = make(chan struct{})
	r.stopped = make(chan struct{})
	r.running = true

	go func() {
		defer close(r.stopped)

		ticker := time.NewTicker(r.every)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, _ = r.Sweep(context.Background())
			case <-r.stop:
				return
			}
		}
	}()

	return r
}

// Stop ends the background sweep and waits for it to finish.
//
// Waiting matters: a caller that stopped the reaper and immediately dropped the
// store would otherwise be racing a sweep that is still holding it.
func (r *Reaper) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	stop, stopped := r.stop, r.stopped
	r.running = false
	r.mu.Unlock()

	close(stop)
	<-stopped
}
