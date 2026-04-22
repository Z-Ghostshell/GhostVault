-- +goose Up
-- +goose StatementBegin

-- Auth-mode dimension on vaults, orthogonal to encryption_enabled.
--   session      – today's RAM-only interactive unlock flow.
--   auto_unlock  – server-side password unwraps the DEK at boot; sessions persist across restarts.
--   api_keys     – per-client DEK-wrapping credentials (future, reserved by CHECK).
--
-- Existing rows default to 'session' so the upgrade is a no-op for current deployments.
ALTER TABLE vaults
    ADD COLUMN auth_mode TEXT NOT NULL DEFAULT 'session'
        CHECK (auth_mode IN ('session','auto_unlock','api_keys'));

-- Persisted sessions. token_hash = sha256(bearer) so the raw token never lives in the DB.
-- DEK is deliberately NOT stored here — it's held in the gvsvd process VaultState and
-- is repopulated at boot by auto_unlock; in pure `session` mode, sessions are purged on
-- gvsvd startup so the behavior matches today's RAM-only semantics.
CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    vault_id   UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    max_until  TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_vault_idx ON sessions (vault_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sessions;
ALTER TABLE vaults DROP COLUMN IF EXISTS auth_mode;
-- +goose StatementEnd
