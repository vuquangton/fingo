package store

import (
	"errors"
	"regexp"
)

// ErrNotUUID is returned for a client id that is not a UUID.
var ErrNotUUID = errors.New("store: not a uuid")

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// parseUUID validates the textual form before it reaches PostgreSQL.
//
// The check is local on purpose: a malformed id should be a programming error
// reported by the caller, not a database error surfaced from inside a
// transaction where it reads like a data problem.
func parseUUID(s string) (string, error) {
	if !uuidPattern.MatchString(s) {
		return "", ErrNotUUID
	}
	return s, nil
}
