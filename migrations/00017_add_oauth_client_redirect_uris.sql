-- +goose Up
ALTER TABLE oauth_clients
    RENAME COLUMN redirect_uri TO redirect_uris;

ALTER TABLE oauth_clients
    ALTER COLUMN redirect_uris TYPE jsonb
    USING CASE WHEN btrim(redirect_uris) = '' THEN '[]'::jsonb ELSE jsonb_build_array(redirect_uris) END;

ALTER TABLE oauth_clients
    ALTER COLUMN redirect_uris SET DEFAULT '[]'::jsonb;

-- +goose Down
-- A single-URI schema can retain only the first registered callback.
ALTER TABLE oauth_clients
    ALTER COLUMN redirect_uris DROP DEFAULT;

ALTER TABLE oauth_clients
    ALTER COLUMN redirect_uris TYPE text
    USING COALESCE(redirect_uris ->> 0, '');

ALTER TABLE oauth_clients
    RENAME COLUMN redirect_uris TO redirect_uri;
