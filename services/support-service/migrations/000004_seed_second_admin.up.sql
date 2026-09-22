-- Deuxième compte admin initial, en plus de celui de la migration 000002.
-- Email    : teourisamrou@gmail.com
-- Password : Admin1234!  (même hash argon2id pré-calculé que 000002)
-- must_change_password = TRUE → la première connexion force ChangeMyPassword.
--
-- Si l'adresse existe déjà (compte créé depuis le back-office, éventuellement
-- avec le rôle support), on garantit le rôle admin et l'activation, sans toucher
-- au mot de passe ni à l'identité du compte existant.
INSERT INTO support_users (
    user_id, email, password_hash, first_name, last_name, role,
    is_active, must_change_password, password_changed_at, created_at, updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000002',
    'teourisamrou@gmail.com',
    '$argon2id$v=19$m=65536,t=1,p=4$nc8mg+FVBJ3LkX7wwSBIIQ$anav5pg1A88NhpB9au5V+gM/e8cYl7qvXplrQNSitPw',
    'Samrou',
    'Teouri',
    'admin',
    TRUE,
    TRUE,
    NOW(),
    NOW(),
    NOW()
)
ON CONFLICT (email) DO UPDATE
    SET role = 'admin', is_active = TRUE, updated_at = NOW();
