-- +goose Up
-- +goose StatementBegin
ALTER TABLE memory_chunks
    ADD COLUMN IF NOT EXISTS ingest_session_key TEXT;

CREATE INDEX IF NOT EXISTS memory_chunks_ingest_session_key_idx
    ON memory_chunks (vault_id, user_id, ingest_session_key)
    WHERE deleted_at IS NULL AND ingest_session_key IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS memory_chunks_ingest_session_key_idx;
ALTER TABLE memory_chunks DROP COLUMN IF EXISTS ingest_session_key;
-- +goose StatementEnd
