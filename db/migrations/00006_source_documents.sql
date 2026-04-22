-- +goose Up
-- +goose StatementBegin
CREATE TABLE source_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    title TEXT,
    ciphertext BYTEA,
    nonce BYTEA,
    key_id TEXT,
    body_text TEXT,
    content_sha256 BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT source_documents_payload_xor CHECK (
        (ciphertext IS NOT NULL AND body_text IS NULL)
        OR (ciphertext IS NULL AND body_text IS NOT NULL)
    )
);

CREATE UNIQUE INDEX source_documents_vault_user_hash_uq
    ON source_documents (vault_id, user_id, content_sha256);

CREATE INDEX source_documents_vault_user_idx
    ON source_documents (vault_id, user_id, created_at DESC);

ALTER TABLE memory_chunks
    ADD COLUMN IF NOT EXISTS source_document_id UUID REFERENCES source_documents (id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE memory_chunks DROP COLUMN IF EXISTS source_document_id;
DROP TABLE IF EXISTS source_documents;
-- +goose StatementEnd
