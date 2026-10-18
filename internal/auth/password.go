// Package auth owns password storage and session wiring.
//
// Nothing here invents cryptography: argon2id comes from golang.org/x/crypto,
// sessions from alexedwards/scs, and the comparison from crypto/subtle. The
// package exists to hold the parameters in one place and to make a wrong
// password cost the same time as a right one.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrMismatch is returned when a password does not match the stored hash.
var ErrMismatch = errors.New("auth: password does not match")

// ErrMalformed is returned for a stored hash this package does not understand.
var ErrMalformed = errors.New("auth: malformed password hash")

// Parameters for argon2id.
//
// These are the RFC 9106 "second recommended option" order of magnitude
// (64 MiB, 3 passes), chosen so a login costs a fraction of a second on an
// office PC while making an offline dictionary attack expensive. They are
// recorded in the hash string, so raising them later does not invalidate
// existing passwords.
const (
	argonMemory  = 64 * 1024 // KiB
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Hash returns a PHC-formatted argon2id hash: $argon2id$v=19$m=...,t=...,p=...$salt$hash.
func Hash(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("auth: empty password")
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches encoded.
//
// The comparison is constant time: a byte-by-byte == would leak how much of the
// hash matched. A malformed stored hash is an error, never a silent success.
func Verify(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrMalformed
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return ErrMalformed
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return ErrMalformed
	}
	if memory == 0 || time == 0 || threads == 0 || memory > 1<<20 {
		return ErrMalformed
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrMalformed
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrMalformed
	}
	if len(want) < 16 || len(want) > 64 {
		return ErrMalformed
	}

	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrMismatch
	}
	return nil
}

// needsRehash reports whether a stored hash was made with weaker parameters
// than the current ones. It is exported for the login path to decide, quietly,
// to upgrade a hash after a successful sign-in.
func needsRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return true
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return true
	}

	return memory < argonMemory || time < argonTime || threads < argonThreads
}
