-- +goose Up
CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Stored lowercase, so the unique constraint ignores case.
    email      text NOT NULL UNIQUE CHECK (email = lower(email)),
    role       text NOT NULL CHECK (role IN ('candidate', 'employer', 'admin')),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE users;
