-- +goose Up
-- A new user picks their role after the first sign-in, so role starts empty.
ALTER TABLE users ALTER COLUMN role DROP NOT NULL;

-- One pending sign-in code per email; a new request replaces the old one.
CREATE TABLE otp_codes (
    email      text PRIMARY KEY CHECK (email = lower(email)),
    code_hash  text NOT NULL,
    attempts   int NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash text NOT NULL UNIQUE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- +goose Down
DROP TABLE sessions;
DROP TABLE otp_codes;
UPDATE users SET role = 'candidate' WHERE role IS NULL;
ALTER TABLE users ALTER COLUMN role SET NOT NULL;
