-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_provider ON user_third_party (user_id, provider);
CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_uid ON user_third_party (provider, provider_uid);

CREATE TABLE IF NOT EXISTS user_oauth_clients (
    id bigserial PRIMARY KEY,
    user_id varchar(36) NOT NULL,
    client_id varchar(50) NOT NULL,
    last_login_at timestamptz NOT NULL,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_oauth_client ON user_oauth_clients (user_id, client_id);

-- +goose Down
DROP TABLE IF EXISTS user_oauth_clients;
DROP INDEX IF EXISTS idx_provider_uid;
DROP INDEX IF EXISTS idx_user_provider;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
