-- +goose Up
-- Employers show their first name on hiring cards.
ALTER TABLE users ADD COLUMN first_name text NOT NULL DEFAULT '';

ALTER TABLE files DROP CONSTRAINT files_kind_check;
ALTER TABLE files ADD CONSTRAINT files_kind_check CHECK (kind IN ('cv', 'photo', 'card_image'));

-- One row per hiring card. An employer can have many; each gets its own
-- candidate timeline. draft: being built; live: shown to candidates;
-- closed: hidden again.
CREATE TABLE hiring_cards (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    employer_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'live', 'closed')),
    title            text NOT NULL DEFAULT '',
    company_name     text NOT NULL DEFAULT '',
    location         text NOT NULL DEFAULT '',
    work_mode        text NOT NULL DEFAULT '' CHECK (work_mode IN ('', 'onsite', 'hybrid', 'remote')),
    seniority        text NOT NULL DEFAULT '',
    skills           jsonb NOT NULL DEFAULT '[]',
    summary          text NOT NULL DEFAULT '',
    -- Yearly pay range in pay_currency; 0 means not given.
    pay_min          integer NOT NULL DEFAULT 0 CHECK (pay_min >= 0),
    pay_max          integer NOT NULL DEFAULT 0 CHECK (pay_max >= 0),
    pay_currency     text NOT NULL DEFAULT 'USD' CHECK (pay_currency ~ '^[A-Z]{3}$'),
    visa_sponsorship boolean NOT NULL DEFAULT false,
    -- The pasted job description, as given.
    description      text NOT NULL DEFAULT '',
    image_file_id    uuid REFERENCES files (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (pay_max = 0 OR pay_max >= pay_min)
);
CREATE INDEX hiring_cards_employer_user_id_idx ON hiring_cards (employer_user_id);
CREATE INDEX hiring_cards_image_file_id_idx ON hiring_cards (image_file_id);

-- +goose Down
DROP TABLE hiring_cards;
DELETE FROM files WHERE kind = 'card_image';
ALTER TABLE files DROP CONSTRAINT files_kind_check;
ALTER TABLE files ADD CONSTRAINT files_kind_check CHECK (kind IN ('cv', 'photo'));
ALTER TABLE users DROP COLUMN first_name;
