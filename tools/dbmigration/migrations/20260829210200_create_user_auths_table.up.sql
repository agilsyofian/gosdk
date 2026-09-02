-- +migrate Up
-- migration: 20260829210200_create_user_auths_table

CREATE TABLE user_auths (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    provider          VARCHAR(50) NOT NULL,
    provider_subject  VARCHAR(255) NOT NULL,
    password_hash     TEXT,
    biometric_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    biometric_hash    TEXT,
    last_login_at     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_user_auths_provider CHECK (provider <> ''),
    CONSTRAINT chk_user_auths_subject CHECK (provider_subject <> ''),
    CONSTRAINT uq_user_auths_provider_subject UNIQUE (provider, provider_subject),
    CONSTRAINT uq_user_auths_user_provider UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_auths_user_id ON user_auths (user_id);
