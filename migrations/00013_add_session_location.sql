-- +goose Up
ALTER TABLE user_session ADD COLUMN location varchar(256) NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE user_session DROP COLUMN location;
