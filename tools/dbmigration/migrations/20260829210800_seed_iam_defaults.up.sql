-- +migrate Up
-- migration: 20260829210800_seed_iam_defaults
-- Baseline only. Adjust role names, permission codes, and mappings for your business.

INSERT INTO iam_roles (name, description) VALUES
    ('super_admin', 'Unrestricted access to all application capabilities.'),
    ('iam_admin', 'Manages roles and permissions.'),
    ('member', 'Standard authenticated user.')
ON CONFLICT (name) DO NOTHING;

INSERT INTO iam_permissions (code, description) VALUES
    ('profile.read', 'View the current user profile.'),
    ('profile.update', 'Update the current user profile.'),
    ('iam.roles.read', 'View roles.'),
    ('iam.roles.write', 'Create, update, or delete roles.'),
    ('iam.permissions.read', 'View permissions.'),
    ('iam.permissions.write', 'Create, update, or delete permissions.')
ON CONFLICT (code) DO NOTHING;

-- super_admin receives every permission currently defined in iam_permissions.
INSERT INTO iam_role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM iam_roles AS roles
CROSS JOIN iam_permissions AS permissions
WHERE roles.name = 'super_admin'
ON CONFLICT DO NOTHING;

INSERT INTO iam_role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM iam_roles AS roles
JOIN iam_permissions AS permissions
    ON permissions.code IN (
        'iam.roles.read',
        'iam.roles.write',
        'iam.permissions.read',
        'iam.permissions.write'
    )
WHERE roles.name = 'iam_admin'
ON CONFLICT DO NOTHING;

INSERT INTO iam_role_permissions (role_id, permission_id)
SELECT roles.id, permissions.id
FROM iam_roles AS roles
JOIN iam_permissions AS permissions
    ON permissions.code IN ('profile.read', 'profile.update')
WHERE roles.name = 'member'
ON CONFLICT DO NOTHING;
