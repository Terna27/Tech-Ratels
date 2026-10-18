-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied

CREATE TABLE IF NOT EXISTS homepage_hero_media (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(255) NOT NULL DEFAULT '',
    video_key VARCHAR(512) NOT NULL,
    video_url TEXT NOT NULL,
    poster_key VARCHAR(512),
    poster_url TEXT,
    content_type VARCHAR(100) NOT NULL DEFAULT 'video/mp4',
    file_size_bytes BIGINT NOT NULL DEFAULT 0,
    duration_seconds NUMERIC(8, 2) DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by_user_id VARCHAR(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    published_by_user_id VARCHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_hero_media_active ON homepage_hero_media(is_active) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_hero_media_created ON homepage_hero_media(created_at DESC);

-- +goose Down
-- SQL in section 'Down' is executed when this migration is applied
DROP TABLE IF EXISTS homepage_hero_media;
