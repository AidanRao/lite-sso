-- +goose Up
ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS logo_url text;
ALTER TABLE oauth_clients ADD COLUMN IF NOT EXISTS logo_object_key varchar(512);

-- +goose Down
ALTER TABLE oauth_clients DROP COLUMN IF EXISTS logo_object_key;
ALTER TABLE oauth_clients DROP COLUMN IF EXISTS logo_url;
