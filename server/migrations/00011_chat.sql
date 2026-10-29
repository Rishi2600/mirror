-- +goose Up
ALTER TABLE files DROP CONSTRAINT files_kind_check;
ALTER TABLE files ADD CONSTRAINT files_kind_check CHECK (kind IN ('cv', 'photo', 'card_image', 'chat_file'));

-- Who ended the match, and when each side last read the chat. A read time of
-- NULL means that side has not opened the match yet (it shows as new).
ALTER TABLE matches
    ADD COLUMN unmatched_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN candidate_read_at timestamptz,
    ADD COLUMN employer_read_at  timestamptz;

-- Chat messages of a match. A message has text, a file, or both. Deleted
-- messages keep their row (and content) for moderation; the API hides them.
CREATE TABLE messages (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id       uuid NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    sender_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    body           text NOT NULL DEFAULT '' CHECK (char_length(body) <= 4000),
    file_id        uuid REFERENCES files (id) ON DELETE SET NULL,
    file_name      text NOT NULL DEFAULT '',
    -- Sent by the client, so a retried send saves the message once.
    client_id      text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz,
    UNIQUE (match_id, client_id)
);
CREATE INDEX messages_match_id_created_at_idx ON messages (match_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE messages;
ALTER TABLE matches
    DROP COLUMN unmatched_by,
    DROP COLUMN candidate_read_at,
    DROP COLUMN employer_read_at;
DELETE FROM files WHERE kind = 'chat_file';
ALTER TABLE files DROP CONSTRAINT files_kind_check;
ALTER TABLE files ADD CONSTRAINT files_kind_check CHECK (kind IN ('cv', 'photo', 'card_image'));
