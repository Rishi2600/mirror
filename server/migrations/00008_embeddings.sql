-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

-- One meaning vector per card and model (see docs/EMBEDDINGS.md, section 9).
-- Vectors from different models must never be compared, so the model is
-- part of the key. halfvec stores each number in 2 bytes.
CREATE TABLE embeddings (
    owner_type text NOT NULL CHECK (owner_type IN ('candidate', 'hiring_card')),
    owner_id   uuid NOT NULL,
    model      text NOT NULL,
    -- SHA-256 of the embedded text: unchanged text is not embedded again.
    text_hash  text NOT NULL,
    embedding  halfvec(384) NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_type, owner_id, model)
);

-- +goose Down
DROP TABLE embeddings;
DROP EXTENSION IF EXISTS vector;
