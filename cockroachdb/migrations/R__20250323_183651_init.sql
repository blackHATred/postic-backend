-- Команда
CREATE TABLE IF NOT EXISTS team (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    name STRING(256) NOT NULL,
    secret STRING(64) NOT NULL DEFAULT gen_random_uuid(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Пользователь
CREATE TABLE IF NOT EXISTS "user" (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY
);

-- Пользователь в команде с ролью
CREATE TABLE IF NOT EXISTS team_user_role (
    team_id INT NOT NULL,
    user_id INT NOT NULL,
    roles STRING(32)[] NOT NULL DEFAULT '{}', -- admin / member
    FOREIGN KEY (team_id) REFERENCES team (id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES "user" (id) ON DELETE CASCADE,
    PRIMARY KEY (team_id, user_id)
);

-- ВКонтакте как канал для постов
CREATE TABLE IF NOT EXISTS channel_vk (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    team_id INT NOT NULL UNIQUE,
    FOREIGN KEY (team_id) REFERENCES team (id) ON DELETE CASCADE,
    group_id INT NOT NULL,
    api_key STRING(256) NOT NULL,
    last_updated_timestamp TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_channel_vk_last_updated_timestamp
ON channel_vk (last_updated_timestamp);

/*
Проверять каналы, которые нужно прослушать, можно так:

SELECT id, last_updated_timestamp
FROM channel_vk
WHERE last_updated_timestamp < NOW() - INTERVAL '5 minutes'
FOR UPDATE;
 */

-- Telegram как канал для постов
CREATE TABLE IF NOT EXISTS channel_tg (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    team_id INT NOT NULL UNIQUE,
    FOREIGN KEY (team_id) REFERENCES team (id) ON DELETE CASCADE,
    channel_id INT NOT NULL UNIQUE,
    discussion_id INT DEFAULT NULL -- NULL, если канал не имеет обсуждений
);

-- Медиа-файлы
CREATE TABLE IF NOT EXISTS mediafile (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    file_path STRING(256) NOT NULL,
    file_type STRING(32) NOT NULL, -- photo / video / raw
    uploaded_by_user_id INT DEFAULT NULL, -- может быть NULL, если файл загружен не пользователем
    FOREIGN KEY (uploaded_by_user_id) REFERENCES "user" (id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Общий пост
CREATE TABLE IF NOT EXISTS post_union (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    user_id INT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES "user" (id) ON DELETE CASCADE,
    team_id INT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES team (id) ON DELETE CASCADE,
    text STRING(1024), -- может быть NULL только в том случае, если есть mediafile
    platforms STRING(32)[] NOT NULL DEFAULT '{}', -- vk / tg / etc
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    pub_datetime TIMESTAMP DEFAULT NULL-- запланированное время публикации; если NULL, то публикуется немедленно
);

-- Прикрепление медиа-файла к посту
CREATE TABLE IF NOT EXISTS post_union_mediafile (
    post_union_id INT NOT NULL,
    mediafile_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id) ON DELETE CASCADE,
    FOREIGN KEY (mediafile_id) REFERENCES mediafile (id) ON DELETE CASCADE,
    PRIMARY KEY (post_union_id, mediafile_id)
);

-- Действие публикации поста
CREATE TABLE IF NOT EXISTS post_action (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id) ON DELETE CASCADE,
    platform STRING(32) NOT NULL, -- vk / tg / etc
    status STRING(32) NOT NULL, -- pending / success / error
    error_message STRING(2048),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Пост во ВКонтакте
CREATE TABLE IF NOT EXISTS post_vk (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id) ON DELETE CASCADE,
    post_id INT NOT NULL
);

-- Пост в Telegram
CREATE TABLE IF NOT EXISTS post_tg (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id) ON DELETE CASCADE,
    post_id INT NOT NULL
);

-- Состояние бота в Telegram
CREATE TABLE IF NOT EXISTS tg_bot_state (
    id INT PRIMARY KEY DEFAULT 1,
    last_update_id INT NOT NULL DEFAULT 0
);

-- Если нет записи для бота, то создаём её
INSERT INTO tg_bot_state (id, last_update_id) VALUES (1, 0) ON CONFLICT DO NOTHING;

-- Комментарий к посту
CREATE TABLE IF NOT EXISTS post_comment (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id) ON DELETE CASCADE,
    platform STRING(32) NOT NULL, -- vk / tg / etc
    post_platform_id INT NOT NULL,
    user_platform_id INT NOT NULL, -- Это юзер, который оставил комментарий и отношения к нашим юзерам не имеет
    comment_platform_id INT NOT NULL,
    full_name STRING(256) NOT NULL,
    username STRING(256) NOT NULL,
    avatar_mediafile_id INT DEFAULT NULL, -- NULL, если нет аватара
    FOREIGN KEY (avatar_mediafile_id) REFERENCES mediafile (id) ON DELETE SET NULL,
    text STRING(4096), -- может быть NULL только в том случае, если есть аттачи
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Attachments к комментарию
CREATE TABLE IF NOT EXISTS post_comment_attachment (
    id INT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    comment_id INT NOT NULL,
    FOREIGN KEY (comment_id) REFERENCES post_comment (id) ON DELETE CASCADE,
    mediafile_id INT NOT NULL,
    FOREIGN KEY (mediafile_id) REFERENCES mediafile (id) ON DELETE CASCADE
);

-- Индекс на comment_id
CREATE INDEX IF NOT EXISTS idx_post_comment_attachment_comment_id ON post_comment_attachment (comment_id);
