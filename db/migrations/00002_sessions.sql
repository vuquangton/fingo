-- +goose Up

-- Sessions live in PostgreSQL so a restart does not sign everybody out, and so
-- the operations team has one thing to back up. This is the schema the scs
-- pgxstore expects; the library owns the queries.
CREATE TABLE sessions (
    token  text PRIMARY KEY,
    data   bytea NOT NULL,
    expiry timestamptz NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- A login attempt that succeeds must be attributable. failed_logins is a
-- practice-level table (no client_id): it records the address and instant of a
-- refusal, never a financial row.
CREATE TABLE failed_logins (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email      text NOT NULL,
    remote_ip  text NOT NULL,
    at         timestamptz NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE IF EXISTS failed_logins;
DROP TABLE IF EXISTS sessions;
