-- +goose Up
-- The browser's time zone, saved at sign-in; the daily like budget resets at
-- local midnight. Empty means UTC.
ALTER TABLE users ADD COLUMN time_zone text NOT NULL DEFAULT '';

-- For candidate swipes: time on the card (measured by the browser) and the
-- fit of the pair (worked out by the server). Older rows have neither.
ALTER TABLE swipes
    ADD COLUMN dwell_ms integer CHECK (dwell_ms >= 0),
    ADD COLUMN fit      real;
CREATE INDEX swipes_actor_user_id_created_at_idx ON swipes (actor_user_id, created_at);

-- One row per candidate who has swiped: the current score, its inputs, and
-- the limits on their likes.
CREATE TABLE selectivity_scores (
    user_id         uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    score           real NOT NULL,
    right_rate      real NOT NULL,
    mean_fit        real NOT NULL,
    median_dwell_ms real NOT NULL,
    reply_rate      real NOT NULL,
    swipes_7d       integer NOT NULL,
    -- When the score went below the low mark; NULL while it is above it.
    low_since       timestamptz,
    paused_until    timestamptz,
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE selectivity_scores;
DROP INDEX swipes_actor_user_id_created_at_idx;
ALTER TABLE swipes
    DROP COLUMN dwell_ms,
    DROP COLUMN fit;
ALTER TABLE users DROP COLUMN time_zone;
