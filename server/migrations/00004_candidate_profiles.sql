-- +goose Up
-- Metadata of uploaded files; the encrypted bytes live in DATA_DIR/files.
CREATE TABLE files (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind          text NOT NULL CHECK (kind IN ('cv')),
    storage_key   text NOT NULL UNIQUE,
    content_type  text NOT NULL,
    size_bytes    bigint NOT NULL CHECK (size_bytes > 0),
    sha256        text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX files_owner_user_id_idx ON files (owner_user_id);

-- One profile per candidate. cv holds what the AI read from the CV (the
-- candidate can edit it); the other columns are the candidate's preferences.
CREATE TABLE candidate_profiles (
    user_id      uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    cv_file_id   uuid REFERENCES files (id) ON DELETE SET NULL,
    parse_status text NOT NULL CHECK (parse_status IN ('pending', 'done', 'failed')),
    parse_error  text NOT NULL DEFAULT '',
    cv           jsonb NOT NULL DEFAULT '{}',
    role_types   jsonb NOT NULL DEFAULT '[]',
    locations    jsonb NOT NULL DEFAULT '[]',
    work_mode    text NOT NULL DEFAULT 'any' CHECK (work_mode IN ('onsite', 'hybrid', 'remote', 'any')),
    -- Minimum yearly pay in pay_currency; 0 means not set.
    min_pay      integer NOT NULL DEFAULT 0 CHECK (min_pay >= 0),
    pay_currency text NOT NULL DEFAULT 'USD' CHECK (pay_currency ~ '^[A-Z]{3}$'),
    needs_visa   boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX candidate_profiles_cv_file_id_idx ON candidate_profiles (cv_file_id);

-- +goose Down
DROP TABLE candidate_profiles;
DROP TABLE files;
