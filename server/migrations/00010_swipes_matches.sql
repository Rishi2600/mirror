-- +goose Up
-- One row per swipe. A candidate swipes on hiring cards; an employer swipes
-- on candidates for one of their cards. Each side swipes once per pair.
CREATE TABLE swipes (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    side              text NOT NULL CHECK (side IN ('candidate', 'employer')),
    actor_user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    hiring_card_id    uuid NOT NULL REFERENCES hiring_cards (id) ON DELETE CASCADE,
    candidate_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    direction         text NOT NULL CHECK (direction IN ('left', 'right')),
    -- Sent by the client, so a retried swipe returns the first result.
    idempotency_key   text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (side, hiring_card_id, candidate_user_id),
    UNIQUE (actor_user_id, idempotency_key)
);
CREATE INDEX swipes_candidate_user_id_idx ON swipes (candidate_user_id);

-- A match: both sides liked the same hiring card and candidate pair.
CREATE TABLE matches (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    hiring_card_id    uuid NOT NULL REFERENCES hiring_cards (id) ON DELETE CASCADE,
    candidate_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    employer_user_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    state             text NOT NULL DEFAULT 'matched' CHECK (state IN ('matched', 'unmatched')),
    matched_at        timestamptz NOT NULL DEFAULT now(),
    unmatched_at      timestamptz,
    UNIQUE (hiring_card_id, candidate_user_id)
);
CREATE INDEX matches_candidate_user_id_idx ON matches (candidate_user_id);
CREATE INDEX matches_employer_user_id_idx ON matches (employer_user_id);

-- Append-only record of who saw or changed what (reveals, CV downloads,
-- unmatches). Needed for consent records and AI audits.
CREATE TABLE audit_events (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    action        text NOT NULL CHECK (action IN ('reveal', 'cv_download', 'unmatch')),
    match_id      uuid REFERENCES matches (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_match_id_idx ON audit_events (match_id);

-- +goose Down
DROP TABLE audit_events;
DROP TABLE matches;
DROP TABLE swipes;
