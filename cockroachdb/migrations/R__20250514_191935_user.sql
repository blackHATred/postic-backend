-- обновление таблицы user: добавляем хэш пароля для bcrypt,
-- имя, email
ALTER TABLE "user"
    ADD COLUMN IF NOT EXISTS password_hash STRING(255),
    ADD COLUMN IF NOT EXISTS nickname STRING(255),
    ADD COLUMN IF NOT EXISTS email STRING(255),
    ADD COLUMN IF NOT EXISTS vk_id INT,
    ADD COLUMN IF NOT EXISTS vk_access_token STRING(255),
    ADD COLUMN IF NOT EXISTS vk_refresh_token STRING(255),
    ADD COLUMN IF NOT EXISTS vk_token_expires_at TIMESTAMPTZ;

ALTER TABLE "user"
    ALTER COLUMN nickname SET DEFAULT '',
    ALTER COLUMN password_hash SET DEFAULT NULL,
    ALTER COLUMN email SET DEFAULT NULL,
    ALTER COLUMN vk_id SET DEFAULT NULL,
    ALTER COLUMN vk_access_token SET DEFAULT NULL,
    ALTER COLUMN vk_refresh_token SET DEFAULT NULL,
    ALTER COLUMN vk_token_expires_at SET DEFAULT NULL;