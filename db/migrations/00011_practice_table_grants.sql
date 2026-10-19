-- +goose Up

-- Migration 00002 created `sessions` and `failed_logins` after 00001's blanket
-- GRANT ON ALL TABLES, and granted nothing itself. On a fresh database the
-- application role therefore had no privilege on the session table, and the first
-- sign-in failed with `permission denied for table sessions` — 500 at login, and a
-- "superfluous WriteHeader" because the session store failed after the handler had
-- already answered.
--
-- The grants live in a migration rather than only in the setup script so that any
-- database, however it was created, ends up with them.
GRANT SELECT, INSERT, UPDATE, DELETE ON sessions TO fingo_app;
GRANT SELECT, INSERT ON failed_logins TO fingo_app;
GRANT SELECT, INSERT, UPDATE ON users TO fingo_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON vat_rates TO fingo_app;

-- +goose Down

-- Privileges are not revoked on the way down: removing them would lock the
-- application out of its own session table.

