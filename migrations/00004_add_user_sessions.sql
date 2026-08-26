-- +goose Up
CREATE TABLE IF NOT EXISTS user_session (
    id varchar(64) PRIMARY KEY,
    user_id varchar(36) NOT NULL,
    device_id varchar(64) NOT NULL,
    auth_method varchar(32) NOT NULL,
    refresh_token_hash char(64) NOT NULL,
    ip varchar(64) NOT NULL,
    user_agent varchar(512) NOT NULL,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revoke_reason varchar(64)
);

CREATE INDEX IF NOT EXISTS idx_user_session_user_id ON user_session (user_id);
CREATE INDEX IF NOT EXISTS idx_user_session_device_id ON user_session (device_id);
CREATE INDEX IF NOT EXISTS idx_user_session_expires_at ON user_session (expires_at);
CREATE INDEX IF NOT EXISTS idx_user_session_revoked_at ON user_session (revoked_at);

-- +goose Down
DROP TABLE IF EXISTS user_session;
