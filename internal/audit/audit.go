// Package audit writes the append-only record of what was done to the books.
//
// The table refuses UPDATE and DELETE, so a correction is another row. A record
// is written inside the same transaction as the change it describes: an audit
// trail that can be one row behind the ledger is not an audit trail.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Action is what happened.
type Action string

const (
	ActionPost   Action = "POST"
	ActionUnpost Action = "UNPOST"
	ActionLock   Action = "PERIOD_LOCK"
	ActionCreate Action = "CREATE"
	ActionUpdate Action = "UPDATE"
)

// Event is one thing that happened.
type Event struct {
	ClientID string
	Actor    string
	Action   Action
	Entity   string
	EntityID string
	Before   []byte // JSON, optional
	After    []byte // JSON, optional
}

// Record appends an event. It must be called with the transaction that performs
// the change, so a rollback removes both.
func Record(ctx context.Context, tx pgx.Tx, e Event) error {
	if e.ClientID == "" || e.Actor == "" || e.Entity == "" || e.EntityID == "" {
		return fmt.Errorf("audit: client, actor, entity and entity id are all required")
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO audit_log (client_id, actor, action, entity, entity_id, before, after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.ClientID, e.Actor, string(e.Action), e.Entity, e.EntityID, e.Before, e.After)
	if err != nil {
		return fmt.Errorf("audit: record %s %s: %w", e.Action, e.Entity, err)
	}
	return nil
}

// Entry is one recorded event, for reading back in tests and screens.
type Entry struct {
	At       time.Time
	Actor    string
	Action   string
	Entity   string
	EntityID string
}
