-- +goose Up
-- One row per LLM call. raw_output can hold CV details, so it is kept only
-- for a short time (a cleanup job comes later).
CREATE TABLE events.ai_calls (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    purpose       text NOT NULL,
    model         text NOT NULL,
    input_tokens  integer NOT NULL DEFAULT 0,
    output_tokens integer NOT NULL DEFAULT 0,
    latency_ms    integer NOT NULL,
    ok            boolean NOT NULL,
    error         text NOT NULL DEFAULT '',
    raw_output    text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE events.ai_calls;
