-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS vector;
-- +goose StatementEnd

CREATE TABLE vaults (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encryption_enabled BOOLEAN NOT NULL,
    argon2_time_cost INT,
    argon2_memory_kb INT,
    argon2_parallelism INT,
    argon2_salt BYTEA,
    wrapped_dek BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE memory_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    ciphertext BYTEA,
    nonce BYTEA,
    key_id TEXT,
    body_text TEXT,
    content_sha256 BYTEA NOT NULL,
    embedding_model_id TEXT NOT NULL,
    chunk_schema_version INT NOT NULL DEFAULT 1,
    search_tsv tsvector,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT memory_chunks_payload_xor CHECK (
        (ciphertext IS NOT NULL AND body_text IS NULL)
        OR (ciphertext IS NULL AND body_text IS NOT NULL)
    )
);

CREATE INDEX memory_chunks_vault_user_created_idx ON memory_chunks (vault_id, user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX memory_chunks_search_tsv_idx ON memory_chunks USING GIN (search_tsv)
    WHERE deleted_at IS NULL;

CREATE TABLE chunk_embeddings (
    chunk_id UUID PRIMARY KEY REFERENCES memory_chunks(id) ON DELETE CASCADE,
    embedding vector(1536) NOT NULL,
    embedding_model_id TEXT NOT NULL
);

CREATE INDEX chunk_embeddings_hnsw_idx ON chunk_embeddings
    USING hnsw (embedding vector_cosine_ops);

CREATE TABLE session_messages (
    id BIGSERIAL PRIMARY KEY,
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    session_key TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX session_messages_session_idx ON session_messages (vault_id, user_id, session_key, created_at DESC);

CREATE TABLE ingest_history (
    id BIGSERIAL PRIMARY KEY,
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    chunk_id UUID REFERENCES memory_chunks(id) ON DELETE SET NULL,
    event TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE action_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    scopes TEXT[] NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX action_tokens_hash_idx ON action_tokens (token_hash);

-- +goose Down
DROP TABLE IF EXISTS action_tokens;
DROP TABLE IF EXISTS ingest_history;
DROP TABLE IF EXISTS session_messages;
DROP TABLE IF EXISTS chunk_embeddings;
DROP TABLE IF EXISTS memory_chunks;
DROP TABLE IF EXISTS vaults;
