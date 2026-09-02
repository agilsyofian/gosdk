-- +migrate Down
-- migration: 20260829232440_create_users_view
-- created_at: 2026-08-29T23:24:40Z

DROP VIEW IF EXISTS v_users CASCADE;