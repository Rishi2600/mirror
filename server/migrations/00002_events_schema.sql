-- +goose Up
-- Event logs live apart from app tables, so they can move to their own database later.
CREATE SCHEMA events;

-- +goose Down
DROP SCHEMA events;
