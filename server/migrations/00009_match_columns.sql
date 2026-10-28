-- +goose Up
-- Columns the timeline filters read for every candidate. Postgres keeps them
-- in step with cv and hidden_from by itself. Reading them is far cheaper
-- than unpacking the whole cv JSON per row (measured: about 240 ms of a
-- 300 ms query at 100,000 candidates).
ALTER TABLE candidate_profiles
    ADD COLUMN seniority text GENERATED ALWAYS AS (coalesce(cv->>'seniority', '')) STORED,
    -- hidden_from in lower case, for case-insensitive company checks.
    ADD COLUMN hidden_keys jsonb GENERATED ALWAYS AS (lower(hidden_from::text)::jsonb) STORED;

-- +goose Down
ALTER TABLE candidate_profiles DROP COLUMN hidden_keys, DROP COLUMN seniority;
