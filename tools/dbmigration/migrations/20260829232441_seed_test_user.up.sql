-- +migrate Up
-- migration: 20260829210900_seed_test_user
-- Development-only account for exercising login, logout, and Bearer middleware.
-- Email:    demo@example.com
-- Password: password (bcrypt hash, password verification is not implemented yet)

INSERT INTO user_profiles (
    id,
    email,
    phone_number,
    full_name,
    display_name
) VALUES (
    '11111111-1111-4111-8111-111111111111',
    'demo@example.com',
    '085808390090',
    'Demo User',
    'demo'
);

INSERT INTO user_auths (
    id,
    user_id,
    provider,
    provider_subject,
    password_hash
) VALUES (
    '22222222-2222-4222-8222-222222222222',
    '11111111-1111-4111-8111-111111111111',
    'local',
    'demo@example.com',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'
);

INSERT INTO user_devices (
    user_id,
    device_identifier,
    platform,
    device_name
) VALUES (
    '11111111-1111-4111-8111-111111111111',
          'LOickgx2ZMRZoMyeIjZAgcfl7p92',
          'ios',
          'Iphone 14|16.7.1'
);

INSERT INTO iam_roles (id, name, description) VALUES
    ('33333333-3333-4333-8333-333333333333', 'user', 'Default role for authenticated users.');

INSERT INTO iam_permissions (id, code, description) VALUES
    ('44444444-4444-4444-8444-444444444444', 'user:read', 'Read the authenticated user.'),
    ('55555555-5555-4555-8555-555555555555', 'user:update', 'Update the authenticated user.');

INSERT INTO iam_user_role_assignments (user_id, role_id)
VALUES (
    '11111111-1111-4111-8111-111111111111',
    '33333333-3333-4333-8333-333333333333'
);

INSERT INTO iam_role_permissions (role_id, permission_id) VALUES
    ('33333333-3333-4333-8333-333333333333', '44444444-4444-4444-8444-444444444444'),
    ('33333333-3333-4333-8333-333333333333', '55555555-5555-4555-8555-555555555555');
