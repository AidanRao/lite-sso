-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    username varchar(50),
    email varchar(100) NOT NULL,
    password_hash varchar(255),
    avatar_url varchar(255),
    is_active boolean DEFAULT true,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);

CREATE TABLE IF NOT EXISTS oauth_clients (
    id bigserial PRIMARY KEY,
    name varchar(50) NOT NULL,
    client_id varchar(50) NOT NULL,
    client_secret varchar(255) NOT NULL,
    redirect_uris text NOT NULL,
    logout_uris text
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_clients_client_id ON oauth_clients (client_id);

CREATE TABLE IF NOT EXISTS user_third_party (
    id bigserial PRIMARY KEY,
    user_id varchar(36) NOT NULL,
    provider varchar(20) NOT NULL,
    provider_uid varchar(100) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_third_party_user_id ON user_third_party (user_id);

-- +goose Down
DROP TABLE IF EXISTS user_third_party;
DROP TABLE IF EXISTS oauth_clients;
DROP TABLE IF EXISTS users;
