-- +migrate Up
-- migration: 20260829210300_create_user_devices_table

CREATE TABLE user_devices (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    device_identifier VARCHAR(255) NOT NULL,
    platform          VARCHAR(30) NOT NULL,
    device_name       VARCHAR(255),
    push_token        TEXT,
    last_seen_at      TIMESTAMPTZ,
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_user_devices_platform CHECK (platform <> ''),
    CONSTRAINT uq_user_devices_identifier UNIQUE (user_id, device_identifier)
);

CREATE INDEX idx_user_devices_user_id ON user_devices (user_id);
CREATE UNIQUE INDEX uq_user_devices_push_token
    ON user_devices (push_token)
    WHERE push_token IS NOT NULL;
