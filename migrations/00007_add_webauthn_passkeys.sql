-- +goose Up
CREATE TABLE webauthn_users (
    id varchar(36) PRIMARY KEY,
    rp_id varchar(255) NOT NULL,
    user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    handle bytea NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT uq_webauthn_users_rp_user UNIQUE (rp_id, user_id),
    CONSTRAINT uq_webauthn_users_rp_handle UNIQUE (rp_id, handle)
);

CREATE TABLE webauthn_credentials (
    id varchar(36) PRIMARY KEY,
    rp_id varchar(255) NOT NULL,
    user_id varchar(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id bytea NOT NULL,
    public_key bytea NOT NULL,
    aaguid bytea,
    attestation_type varchar(64) NOT NULL,
    attestation_format varchar(64) NOT NULL,
    transports_json text NOT NULL,
    attachment varchar(32) NOT NULL,
    flags smallint NOT NULL,
    sign_count bigint NOT NULL,
    clone_warning boolean NOT NULL DEFAULT false,
    attestation_client_data bytea,
    attestation_client_hash bytea,
    attestation_auth_data bytea,
    public_key_algorithm bigint NOT NULL,
    attestation_object bytea,
    extensions_json text NOT NULL DEFAULT '{}',
    name varchar(64) NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    last_used_at timestamptz,
    CONSTRAINT uq_webauthn_credentials_rp_id UNIQUE (rp_id, credential_id)
);

CREATE INDEX idx_webauthn_credentials_rp_user ON webauthn_credentials (rp_id, user_id);
CREATE INDEX idx_webauthn_credentials_last_used ON webauthn_credentials (last_used_at);

-- +goose Down
DROP TABLE IF EXISTS webauthn_credentials;
DROP TABLE IF EXISTS webauthn_users;
