-- +goose Up
ALTER TABLE users ADD COLUMN status varchar(24) NOT NULL DEFAULT 'active';
UPDATE users SET status = 'disabled' WHERE NOT COALESCE(is_active, false);
ALTER TABLE users DROP COLUMN is_active;
ALTER TABLE user_session ADD COLUMN purpose varchar(32) NOT NULL DEFAULT 'normal';
ALTER TABLE user_session ADD COLUMN deletion_id varchar(36);
CREATE TABLE user_deletions (
 id varchar(36) PRIMARY KEY,
 user_id varchar(36) NOT NULL,
 requested_at timestamptz NOT NULL,
 scheduled_at timestamptz NOT NULL,
 status varchar(16) NOT NULL,
 finished_at timestamptz
);
CREATE INDEX idx_user_deletions_user_id ON user_deletions (user_id, requested_at DESC);
CREATE INDEX idx_user_deletions_scheduled_at ON user_deletions (scheduled_at) WHERE status = 'pending';
CREATE TABLE user_cleanup_tasks (
 id varchar(36) PRIMARY KEY,
 user_id varchar(36) NOT NULL,
 kind varchar(32) NOT NULL,
 object_key varchar(512) NOT NULL DEFAULT '',
 cutoff timestamptz NOT NULL,
 attempts integer NOT NULL DEFAULT 0,
 next_attempt_at timestamptz NOT NULL
);
CREATE INDEX idx_user_cleanup_tasks_next_attempt_at ON user_cleanup_tasks (next_attempt_at);

-- +goose Down
ALTER TABLE users ADD COLUMN is_active boolean DEFAULT true;
UPDATE users SET is_active = (status = 'active');
DROP TABLE user_cleanup_tasks;
DROP TABLE user_deletions;
ALTER TABLE user_session DROP COLUMN deletion_id;
ALTER TABLE user_session DROP COLUMN purpose;
ALTER TABLE users DROP COLUMN status;
