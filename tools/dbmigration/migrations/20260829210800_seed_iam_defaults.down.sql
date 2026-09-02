-- +migrate Down
-- migration: 20260829210800_seed_iam_defaults

DELETE FROM iam_role_permissions
WHERE role_id IN (
    SELECT id FROM iam_roles
    WHERE name IN ('super_admin', 'iam_admin', 'member')
);

DELETE FROM iam_permissions
WHERE code IN (
    'profile.read',
    'profile.update',
    'iam.roles.read',
    'iam.roles.write',
    'iam.permissions.read',
    'iam.permissions.write'
);

DELETE FROM iam_roles
WHERE name IN ('super_admin', 'iam_admin', 'member');
