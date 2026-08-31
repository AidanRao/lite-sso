-- +goose Up
CREATE TABLE audit_logs (
    id uuid PRIMARY KEY,
    occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    user_id varchar(36),
    actor_type varchar(16) NOT NULL,
    action varchar(64) NOT NULL,
    target_type varchar(32) NOT NULL,
    target_id varchar(128) NOT NULL,
    client_id varchar(64),
    method varchar(16) NOT NULL,
    route varchar(160) NOT NULL,
    http_status integer NOT NULL,
    outcome varchar(16) NOT NULL,
    reason_code varchar(64) NOT NULL,
    duration_ms bigint NOT NULL,
    ip varchar(64) NOT NULL,
    device_id varchar(64) NOT NULL,
    session_hash char(64) NOT NULL,
    device_label varchar(32) NOT NULL,
    details jsonb NOT NULL,
    CONSTRAINT audit_logs_actor_type CHECK (actor_type IN ('user', 'admin', 'anonymous')),
    CONSTRAINT audit_logs_outcome CHECK (outcome IN ('success', 'failure', 'denied'))
);

CREATE INDEX idx_audit_logs_time ON audit_logs (occurred_at, id);
CREATE INDEX idx_audit_logs_user_time ON audit_logs (user_id, occurred_at, id);
CREATE INDEX idx_audit_logs_client_time ON audit_logs (client_id, occurred_at, id);

-- +goose Down
DROP TABLE audit_logs;
