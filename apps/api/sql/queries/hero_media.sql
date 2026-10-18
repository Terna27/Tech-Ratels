-- name: GetActiveHeroMedia :one
SELECT * FROM homepage_hero_media
WHERE is_active = TRUE
ORDER BY published_at DESC NULLS LAST, created_at DESC
LIMIT 1;

-- name: ListHeroMedia :many
SELECT * FROM homepage_hero_media
ORDER BY created_at DESC;

-- name: GetHeroMediaByID :one
SELECT * FROM homepage_hero_media
WHERE id = $1;

-- name: GetLatestPublishedHeroMedia :one
SELECT * FROM homepage_hero_media
WHERE published_at IS NOT NULL
ORDER BY published_at DESC
LIMIT 1;

-- name: CreateHeroMedia :one
INSERT INTO homepage_hero_media (
    id, title, video_key, video_url, poster_key, poster_url,
    content_type, file_size_bytes, duration_seconds, is_active,
    created_by_user_id, published_by_user_id, created_at, published_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, CURRENT_TIMESTAMP, $13, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: DeactivateAllHeroMedia :exec
UPDATE homepage_hero_media
SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP
WHERE is_active = TRUE;

-- name: PublishHeroMedia :one
UPDATE homepage_hero_media
SET is_active = TRUE,
    published_by_user_id = $2,
    published_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: ToggleHeroMediaPlayback :one
UPDATE homepage_hero_media
SET is_active = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateHeroMediaPoster :one
UPDATE homepage_hero_media
SET poster_key = $2,
    poster_url = $3,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: DeleteHeroMedia :exec
DELETE FROM homepage_hero_media
WHERE id = $1;
