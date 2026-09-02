-- +migrate Up
-- migration: 20260829210500_create_permissions_table

CREATE TABLE iam_permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(150) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_iam_permissions_code CHECK (code <> ''),
    CONSTRAINT uq_iam_permissions_code UNIQUE (code)
);
