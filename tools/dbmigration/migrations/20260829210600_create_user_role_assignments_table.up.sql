-- +migrate Up
-- migration: 20260829210600_create_user_role_assignments_table

CREATE TABLE iam_user_role_assignments (
    user_id     UUID NOT NULL REFERENCES user_profiles(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES iam_roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_iam_user_role_assignments_role_id
    ON iam_user_role_assignments (role_id);
