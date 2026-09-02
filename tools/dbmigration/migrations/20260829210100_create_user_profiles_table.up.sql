-- +migrate Up
-- migration: 20260829210100_create_user_profiles_table

CREATE TABLE user_profiles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status        VARCHAR(20) NOT NULL DEFAULT 'active',
    email         VARCHAR(255) UNIQUE NOT NULL,
    phone_number  VARCHAR(32) UNIQUE NOT NULL,
    full_name     VARCHAR(255) NOT NULL,
    display_name  VARCHAR(100),
    avatar_url    TEXT,
    locale        VARCHAR(16),
    timezone      VARCHAR(64),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT chk_user_profiles_status CHECK (status IN ('active', 'suspended', 'disabled'))
);

CREATE UNIQUE INDEX uq_user_profiles_email_active
    ON user_profiles (LOWER(email))
    WHERE deleted_at IS NULL;
