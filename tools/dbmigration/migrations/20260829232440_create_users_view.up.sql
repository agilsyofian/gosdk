-- +migrate Up
-- migration: 20260829232440_create_users_view
-- created_at: 2026-08-29T23:24:40Z

CREATE OR REPLACE VIEW v_users AS
SELECT
    p.id,
    p.status,
    p.email,
    p.phone_number,
    p.full_name,
    p.display_name,
    p.avatar_url,
    p.locale,
    p.timezone,
    ua.provider_subject as subject,
    ua.password_hash,
    ua.biometric_enabled,
    ua.biometric_hash,
    COALESCE(
            ARRAY(
                SELECT DISTINCT ir.name
                 FROM iam_user_role_assignments AS ura
                          JOIN iam_roles AS ir ON ir.id = ura.role_id
                 WHERE ura.user_id = p.id
                 ORDER BY ir.name
         ), ARRAY[]::VARCHAR[]
    ) as roles,
    COALESCE(
            ARRAY(
                SELECT DISTINCT ip.code
                 FROM iam_user_role_assignments AS ura
                          JOIN iam_role_permissions AS irp ON irp.role_id = ura.role_id
                          JOIN iam_permissions AS ip ON ip.id = irp.permission_id
                 WHERE ura.user_id = p.id
                 ORDER BY ip.code
         ), ARRAY[]::VARCHAR[]
    ) as permissions,
    d.device_identifier,
    d.platform,
    d.device_name,
    d.push_token
FROM
    user_profiles AS p
        LEFT JOIN user_auths AS ua ON ua.user_id = p.id
        LEFT JOIN user_devices AS d ON d.user_id = p.id
WHERE p.deleted_at IS NULL
