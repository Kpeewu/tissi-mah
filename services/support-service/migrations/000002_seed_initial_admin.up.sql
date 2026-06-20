-- Compte admin initial.
-- Email    : admin@tissimah.local
-- Password : Admin1234!  (hash argon2id pré-calculé)
-- must_change_password = TRUE → la première connexion force ChangeMyPassword.
INSERT INTO support_users (
    user_id, email, password_hash, first_name, last_name, role,
    is_active, must_change_password, password_changed_at, created_at, updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'ttoureydaou@gmail.com',
    '$argon2id$v=19$m=65536,t=1,p=4$nc8mg+FVBJ3LkX7wwSBIIQ$anav5pg1A88NhpB9au5V+gM/e8cYl7qvXplrQNSitPw',
    'Admin',
    'TissiMah',
    'admin',
    TRUE,
    TRUE,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;
