-- обновление таблицы user: добавляем хэш пароля для bcrypt,
-- имя, email
ALTER TABLE "user"
ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255) DEFAULT NULL,
ADD COLUMN IF NOT EXISTS nickname VARCHAR(255) DEFAULT NULL,
ADD COLUMN IF NOT EXISTS email VARCHAR(255) DEFAULT NULL;
