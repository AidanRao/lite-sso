-- +goose Up
ALTER TABLE oauth_clients
    RENAME COLUMN redirect_uris TO redirect_uri;

ALTER TABLE oauth_clients
    DROP COLUMN logout_uris;

ALTER TABLE oauth_clients
    ADD COLUMN homepage_url text NOT NULL DEFAULT '';

ALTER TABLE oauth_clients
    ADD COLUMN logout_uri text;

ALTER TABLE oauth_clients
    ALTER COLUMN homepage_url DROP DEFAULT;

-- +goose Down
ALTER TABLE oauth_clients
    DROP COLUMN logout_uri;

ALTER TABLE oauth_clients
    DROP COLUMN homepage_url;

ALTER TABLE oauth_clients
    ADD COLUMN logout_uris text;

ALTER TABLE oauth_clients
    RENAME COLUMN redirect_uri TO redirect_uris;
