-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_object_key varchar(512);

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS avatar_object_key;
