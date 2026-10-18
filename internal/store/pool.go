package store

import "github.com/jackc/pgx/v5/pgxpool"

// FromPool wraps a pool the caller already opened.
//
// It exists so the HTTP layer and the posting service share one pool rather than
// opening a second one: two pools means two sets of connections, two lifetimes,
// and a shutdown order that has to be right twice.
func FromPool(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}
