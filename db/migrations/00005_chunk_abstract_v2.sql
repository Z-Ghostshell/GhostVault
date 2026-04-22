-- +goose Up
-- +goose StatementBegin
ALTER TABLE memory_chunks
  ADD COLUMN IF NOT EXISTS item_kind TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS item_metadata JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE memory_chunks
  DROP COLUMN IF EXISTS item_metadata,
  DROP COLUMN IF EXISTS item_kind;
-- +goose StatementEnd
