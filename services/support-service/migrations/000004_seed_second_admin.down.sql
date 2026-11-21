-- Ne supprime que le compte inséré par la migration (identifiant fixe), pour ne
-- pas toucher un compte homonyme créé depuis le back-office.
DELETE FROM support_users WHERE user_id = '00000000-0000-0000-0000-000000000002';
