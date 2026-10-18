// Package db carries the schema migrations and the runner that applies them.
package db

import "embed"

// Migrations holds every schema migration, and the triggers that go with them.
// The files are embedded so a deployed binary needs no loose .sql files beside
// it.
//
//go:embed migrations/*.sql triggers/*.sql
var Migrations embed.FS
