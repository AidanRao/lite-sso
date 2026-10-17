-- +goose Up
ALTER TABLE user_third_party ADD COLUMN bound_at timestamptz;
ALTER TABLE user_third_party ADD COLUMN last_used_at timestamptz;

-- +goose Down
ALTER TABLE user_third_party DROP COLUMN last_used_at;
ALTER TABLE user_third_party DROP COLUMN bound_at;
