-- +goose Up
ALTER TABLE user_third_party ADD COLUMN email text NOT NULL DEFAULT '';
ALTER TABLE user_third_party ADD COLUMN username text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE user_third_party DROP COLUMN username;
ALTER TABLE user_third_party DROP COLUMN email;
