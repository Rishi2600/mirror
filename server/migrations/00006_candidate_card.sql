-- +goose Up
ALTER TABLE files DROP CONSTRAINT files_kind_check;
ALTER TABLE files ADD CONSTRAINT files_kind_check CHECK (kind IN ('cv', 'photo'));

ALTER TABLE candidate_profiles
    ADD COLUMN photo_file_id uuid REFERENCES files (id) ON DELETE SET NULL,
    -- How the card shows the current employer: its name, the candidate's own
    -- description of it, or nothing.
    ADD COLUMN employer_display text NOT NULL DEFAULT 'name'
        CHECK (employer_display IN ('name', 'description', 'hidden')),
    ADD COLUMN employer_description text NOT NULL DEFAULT '',
    ADD COLUMN show_pay boolean NOT NULL DEFAULT true,
    -- Company names the card is hidden from (the current employer is added
    -- when the CV is read).
    ADD COLUMN hidden_from jsonb NOT NULL DEFAULT '[]',
    -- Set when the candidate finishes building the card; from then on the
    -- card is live for employers.
    ADD COLUMN onboarded_at timestamptz;
CREATE INDEX candidate_profiles_photo_file_id_idx ON candidate_profiles (photo_file_id);

-- +goose Down
ALTER TABLE candidate_profiles
    DROP COLUMN onboarded_at,
    DROP COLUMN hidden_from,
    DROP COLUMN show_pay,
    DROP COLUMN employer_description,
    DROP COLUMN employer_display,
    DROP COLUMN photo_file_id;
DELETE FROM files WHERE kind = 'photo';
ALTER TABLE files DROP CONSTRAINT files_kind_check;
ALTER TABLE files ADD CONSTRAINT files_kind_check CHECK (kind IN ('cv'));
