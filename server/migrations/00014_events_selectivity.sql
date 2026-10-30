-- +goose Up
-- History of each candidate's score and its inputs, for tuning and audits.
CREATE TABLE events.selectivity_snapshots (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id         uuid NOT NULL,
    score           real NOT NULL,
    right_rate      real NOT NULL,
    mean_fit        real NOT NULL,
    median_dwell_ms real NOT NULL,
    reply_rate      real NOT NULL,
    swipes_7d       integer NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE events.selectivity_snapshots;
