-- +goose Up
-- Empty identities preserve grants belonging to existing registrations and associations.
ALTER TABLE oauth_clients ADD COLUMN registration_id varchar(36) NOT NULL DEFAULT '';
ALTER TABLE user_oauth_clients ADD COLUMN authorization_id varchar(36) NOT NULL DEFAULT '';
ALTER TABLE oauth_clients ALTER COLUMN registration_id DROP DEFAULT;
ALTER TABLE user_oauth_clients ALTER COLUMN authorization_id DROP DEFAULT;
CREATE INDEX idx_user_oauth_clients_page ON user_oauth_clients (client_id, last_login_at DESC, user_id);

-- +goose Down
DROP INDEX idx_user_oauth_clients_page;
ALTER TABLE user_oauth_clients DROP COLUMN authorization_id;
ALTER TABLE oauth_clients DROP COLUMN registration_id;
