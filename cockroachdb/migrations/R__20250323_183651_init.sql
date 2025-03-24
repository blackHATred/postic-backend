-- Команды
CREATE TABLE IF NOT EXISTS team (
    id INT PRIMARY KEY,
    name STRING(128) NOT NULL,
    secret_token UUID NOT NULL DEFAULT gen_random_uuid(),
);

-- Пользователь
CREATE TABLE IF NOT EXISTS user (
    id INT PRIMARY KEY,
);

-- Член команды
CREATE TABLE IF NOT EXISTS team_member (
    team_id INT NOT NULL,
    user_id INT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES team (id),
    FOREIGN KEY (user_id) REFERENCES user (id),
    roles STRING(128)[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (team_id, user_id),
);

-- ВКонтакте как канал для постов
CREATE TABLE IF NOT EXISTS channel_vk (
    id INT PRIMARY KEY,
    team_id INT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES team (id),
    group_id INT NOT NULL,
    api_key STRING(128) NOT NULL,
    last_updated_timestamp TIMESTAMP,
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
    id INT PRIMARY KEY,
    team_id INT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES team (id),
    channel_id INT NOT NULL,
    discussion_id INT, -- NULL, если канал не имеет обсуждений
);

-- Медиа-файлы
CREATE TABLE IF NOT EXISTS mediafile (
    id INT PRIMARY KEY,
    file_path STRING(256) NOT NULL,
    file_type STRING(32) NOT NULL, -- photo / video / raw
    uploaded_by_user_id INT NOT NULL,
    FOREIGN KEY (uploaded_by_user_id) REFERENCES user (id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
);

-- Общий пост
CREATE TABLE IF NOT EXISTS post_union (
    id INT PRIMARY KEY,
    team_id INT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES team (id),
    user_id INT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user (id),
    text STRING(1024), -- может быть NULL только в том случае, если есть mediafile
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    pud_datetime TIMESTAMP, -- запланированное время публикации; если NULL, то публикуется немедленно
);

-- Прикрепление медиа-файла к посту
CREATE TABLE IF NOT EXISTS post_union_mediafile (
    post_union_id INT NOT NULL,
    mediafile_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id),
    FOREIGN KEY (mediafile_id) REFERENCES mediafile (id),
    PRIMARY KEY (post_union_id, mediafile_id),
);

-- Действие публикации поста в ВКонтакте
CREATE TABLE IF NOT EXISTS action_post_vk (
    id INT PRIMARY KEY,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id),
    status STRING(32) NOT NULL, -- pending / success / error
    error_message STRING(256),
);

-- Пост в ВКонтакте
CREATE TABLE IF NOT EXISTS post_vk (
    id INT PRIMARY KEY,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union_id (id),
    post_id INT NOT NULL,
);
