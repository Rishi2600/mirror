-- +goose Up
-- One row per in-app notification (a new match or message). delivered says
-- whether the user had the app open to receive it live.
CREATE TABLE events.notifications (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    uuid NOT NULL,
    channel    text NOT NULL,
    kind       text NOT NULL,
    match_id   uuid,
    delivered  boolean NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE events.notifications;
