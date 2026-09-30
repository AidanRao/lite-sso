-- +goose Up
ALTER TABLE oauth_clients
    ADD COLUMN audiences jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN allowed_scopes jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE oauth_clients
    DROP COLUMN allowed_scopes,
    DROP COLUMN audiences;
