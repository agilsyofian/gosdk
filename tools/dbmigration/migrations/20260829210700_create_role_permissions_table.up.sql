-- +migrate Up
-- migration: 20260829210700_create_role_permissions_table

CREATE TABLE iam_role_permissions (
    role_id       UUID NOT NULL REFERENCES iam_roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES iam_permissions(id) ON DELETE CASCADE,
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_iam_role_permissions_permission_id
    ON iam_role_permissions (permission_id);
