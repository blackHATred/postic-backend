-- Хранит статистику по постам в разных платформах
CREATE TABLE IF NOT EXISTS post_platform_stats (
    team_id INT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES team (id) ON DELETE CASCADE,
    post_union_id INT NOT NULL,
    FOREIGN KEY (post_union_id) REFERENCES post_union (id) ON DELETE CASCADE,
    platform STRING(32) NOT NULL, -- vk / tg / etc
    views INT NOT NULL DEFAULT 0,
    reactions INT NOT NULL DEFAULT 0,
    -- comments не нужны, их считаем сами
    last_update TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (post_union_id, platform)
);

