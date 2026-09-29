-- +goose Up
ALTER TABLE oauth_clients
    ADD COLUMN client_type varchar(20) NOT NULL DEFAULT 'confidential';

-- +goose Down
ALTER TABLE oauth_clients DROP COLUMN client_type;
