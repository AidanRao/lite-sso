-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM users
        WHERE email IS NOT NULL AND btrim(email) <> ''
        GROUP BY lower(btrim(email))
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'users.email contains case-insensitive duplicates; user_emails migration aborted';
    END IF;
END $$;
-- +goose StatementEnd

CREATE TABLE user_emails (
    id varchar(36) PRIMARY KEY,
    user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email varchar(100) NOT NULL,
    verified_at timestamptz,
    is_primary boolean NOT NULL DEFAULT false,
    verification_token_hash char(64),
    verification_expires_at timestamptz,
    verification_sent_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT uq_user_emails_user_email UNIQUE (user_id, email)
);

CREATE INDEX idx_user_emails_user_id ON user_emails (user_id);
CREATE UNIQUE INDEX uq_user_emails_verified_email ON user_emails (email) WHERE verified_at IS NOT NULL;
CREATE UNIQUE INDEX uq_user_emails_primary ON user_emails (user_id) WHERE is_primary;
CREATE UNIQUE INDEX uq_user_emails_verification_token ON user_emails (verification_token_hash) WHERE verification_token_hash IS NOT NULL;

CREATE TABLE user_email_sources (
    id bigserial PRIMARY KEY,
    user_email_id varchar(36) NOT NULL REFERENCES user_emails(id) ON DELETE CASCADE,
    user_third_party_id bigint NOT NULL REFERENCES user_third_party(id) ON DELETE CASCADE,
    CONSTRAINT uq_user_email_sources_binding UNIQUE (user_email_id, user_third_party_id)
);

CREATE INDEX idx_user_email_sources_email_id ON user_email_sources (user_email_id);
CREATE INDEX idx_user_email_sources_binding_id ON user_email_sources (user_third_party_id);

INSERT INTO user_emails (
    id,
    user_id,
    email,
    verified_at,
    is_primary,
    created_at,
    updated_at
)
SELECT
    'ue_' || md5(id),
    id,
    lower(btrim(email)),
    COALESCE(updated_at, created_at, CURRENT_TIMESTAMP),
    true,
    COALESCE(created_at, CURRENT_TIMESTAMP),
    COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
FROM users
WHERE email IS NOT NULL
  AND btrim(email) <> '';

DROP INDEX IF EXISTS idx_users_email;
ALTER TABLE users DROP COLUMN email;

-- +goose Down
ALTER TABLE users ADD COLUMN email varchar(100);

UPDATE users
SET email = user_emails.email
FROM user_emails
WHERE user_emails.user_id = users.id
  AND user_emails.is_primary = true;

CREATE UNIQUE INDEX idx_users_email ON users (email);

DROP TABLE IF EXISTS user_email_sources;
DROP TABLE IF EXISTS user_emails;
