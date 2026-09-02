-- +migrate Up
-- migration: 20260829210400_create_roles_table

CREATE TABLE iam_roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_iam_roles_name CHECK (name <> ''),
    CONSTRAINT uq_iam_roles_name UNIQUE (name)
);
