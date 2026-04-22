-- +goose Up
CREATE TABLE server_tuning (
    id INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    overrides JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO server_tuning (id, overrides) VALUES (1, '{}')
    ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS server_tuning;
