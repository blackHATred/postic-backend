-- +goose Up
-- Добавляем поле для хранения timestamp последнего обработанного события VK
ALTER TABLE channel_vk ADD COLUMN IF NOT EXISTS last_event_ts STRING DEFAULT '0';

-- Создаём индекс для ускорения поиска по team_id
CREATE INDEX IF NOT EXISTS idx_channel_vk_team_id
ON channel_vk (team_id);
