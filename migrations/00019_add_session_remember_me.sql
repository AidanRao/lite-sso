-- +goose Up
-- Revoke every existing browser session before introducing the explicit persistence policy.
UPDATE user_session SET revoked_at = CURRENT_TIMESTAMP, revoke_reason = 'session_policy_changed' WHERE revoked_at IS NULL;
ALTER TABLE user_session ADD COLUMN remember_me boolean NOT NULL DEFAULT false;
ALTER TABLE user_session ALTER COLUMN remember_me DROP DEFAULT;

-- +goose Down
ALTER TABLE user_session DROP COLUMN remember_me;
