-- +goose Up
CREATE TABLE feature_configs (
    key varchar(128) PRIMARY KEY,
    audience varchar(16) NOT NULL DEFAULT 'off' CHECK (audience IN ('off', 'selected', 'percentage', 'all')),
    percentage integer NOT NULL DEFAULT 0 CHECK (percentage BETWEEN 0 AND 100),
    stage varchar(16) NOT NULL DEFAULT 'beta' CHECK (stage IN ('beta', 'stable'))
);
CREATE TABLE feature_users (
    feature_key varchar(128) NOT NULL REFERENCES feature_configs(key) ON DELETE CASCADE,
    user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (feature_key, user_id)
);
INSERT INTO feature_configs (key, audience, percentage, stage)
VALUES ('profile.audit_logs', 'all', 100, 'stable');

-- +goose Down
DROP TABLE feature_users;
DROP TABLE feature_configs;
