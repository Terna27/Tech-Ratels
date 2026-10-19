-- ============================================================================
-- Event Media Gallery & Moderation SQLC Queries (Phase A)
-- ============================================================================

-- name: CreateEventMedia :one
INSERT INTO event_media (
    id,
    event_id,
    uploaded_by_user_id,
    media_type,
    caption,
    description,
    storage_key,
    public_url,
    thumbnail_url,
    mime_type,
    file_size_bytes,
    width,
    height,
    duration_seconds,
    moderation_status,
    rejection_reason,
    reviewed_by_user_id,
    reviewed_at,
    is_published,
    published_at,
    is_hidden,
    is_featured,
    metadata_sanitized,
    sanitized_at,
    allow_public_attribution,
    state_id,
    lga_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27
)
RETURNING *;

-- name: GetEventMediaByID :one
SELECT * FROM event_media
WHERE id = $1;

-- name: GetEventMediaDetailForPublic :one
SELECT 
    m.id,
    m.event_id,
    m.media_type,
    m.caption,
    m.description,
    m.public_url,
    m.thumbnail_url,
    m.mime_type,
    m.file_size_bytes,
    m.width,
    m.height,
    m.duration_seconds,
    m.published_at,
    m.created_at,
    m.allow_public_attribution,
    CASE WHEN m.allow_public_attribution THEN u.display_name ELSE NULL END AS uploader_display_name,
    e.title AS event_title,
    e.start_time AS event_date,
    s.name AS state_name,
    s.code AS state_code,
    l.name AS lga_name,
    z.id AS zone_id,
    z.name AS zone_name
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN users u ON m.uploaded_by_user_id = u.id
JOIN states s ON m.state_id = s.id
JOIN lgas l ON m.lga_id = l.id
JOIN zones z ON s.zone_id = z.id
WHERE m.id = $1 
  AND m.is_published = TRUE 
  AND m.is_hidden = FALSE 
  AND m.moderation_status = 'approved';

-- name: GetEventMediaDetailForAdmin :one
SELECT 
    m.id,
    m.event_id,
    m.uploaded_by_user_id,
    m.media_type,
    m.caption,
    m.description,
    m.storage_key,
    m.public_url,
    m.thumbnail_url,
    m.mime_type,
    m.file_size_bytes,
    m.width,
    m.height,
    m.duration_seconds,
    m.moderation_status,
    m.rejection_reason,
    m.reviewed_by_user_id,
    m.reviewed_at,
    m.is_published,
    m.published_at,
    m.is_hidden,
    m.is_featured,
    m.metadata_sanitized,
    m.sanitized_at,
    m.allow_public_attribution,
    m.state_id,
    m.lga_id,
    m.created_at,
    m.updated_at,
    m.version,
    u.display_name AS uploader_name,
    u.email AS uploader_email,
    e.title AS event_title,
    e.start_time AS event_date,
    s.name AS state_name,
    l.name AS lga_name,
    z.id AS zone_id,
    z.name AS zone_name,
    rev.display_name AS reviewer_name
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN users u ON m.uploaded_by_user_id = u.id
JOIN states s ON m.state_id = s.id
JOIN lgas l ON m.lga_id = l.id
JOIN zones z ON s.zone_id = z.id
LEFT JOIN users rev ON m.reviewed_by_user_id = rev.id
WHERE m.id = $1;

-- name: CreateEventMediaConsent :one
INSERT INTO event_media_consent (
    id,
    media_id,
    has_owner_permission,
    understands_public_display,
    has_identifiable_consent,
    has_no_confidential_info,
    has_minors,
    has_guardian_consent,
    consent_notes,
    declared_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetEventMediaConsentByMediaID :one
SELECT * FROM event_media_consent
WHERE media_id = $1;

-- name: ApproveEventMedia :one
UPDATE event_media
SET moderation_status = 'approved',
    public_url = $2,
    thumbnail_url = $3,
    is_published = TRUE,
    published_at = CURRENT_TIMESTAMP,
    metadata_sanitized = TRUE,
    sanitized_at = CURRENT_TIMESTAMP,
    reviewed_by_user_id = $4,
    reviewed_at = CURRENT_TIMESTAMP,
    rejection_reason = NULL,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: RejectEventMedia :one
UPDATE event_media
SET moderation_status = 'rejected',
    rejection_reason = $2,
    reviewed_by_user_id = $3,
    reviewed_at = CURRENT_TIMESTAMP,
    is_published = FALSE,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: RequestRevisionEventMedia :one
UPDATE event_media
SET moderation_status = 'revision_requested',
    rejection_reason = $2,
    reviewed_by_user_id = $3,
    reviewed_at = CURRENT_TIMESTAMP,
    is_published = FALSE,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: WithdrawEventMedia :one
UPDATE event_media
SET moderation_status = 'withdrawn',
    rejection_reason = $2,
    is_published = FALSE,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1 AND uploaded_by_user_id = $3
RETURNING *;

-- name: RemoveEventMedia :one
UPDATE event_media
SET moderation_status = 'removed',
    is_published = FALSE,
    is_hidden = TRUE,
    rejection_reason = $2,
    reviewed_by_user_id = $3,
    reviewed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: ToggleHideEventMedia :one
UPDATE event_media
SET is_hidden = $2,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: ListPublicGalleryMedia :many
SELECT 
    m.id,
    m.event_id,
    m.media_type,
    m.caption,
    m.description,
    m.public_url,
    m.thumbnail_url,
    m.mime_type,
    m.file_size_bytes,
    m.width,
    m.height,
    m.duration_seconds,
    m.published_at,
    m.created_at,
    m.is_featured,
    m.allow_public_attribution,
    CASE WHEN m.allow_public_attribution THEN u.display_name ELSE NULL END AS uploader_display_name,
    e.title AS event_title,
    e.start_time AS event_date,
    s.name AS state_name,
    s.code AS state_code,
    l.name AS lga_name,
    z.id AS zone_id,
    z.name AS zone_name
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN users u ON m.uploaded_by_user_id = u.id
JOIN states s ON m.state_id = s.id
JOIN lgas l ON m.lga_id = l.id
JOIN zones z ON s.zone_id = z.id
WHERE m.is_published = TRUE 
  AND m.is_hidden = FALSE 
  AND m.moderation_status = 'approved'
  AND (sqlc.narg('media_type')::varchar IS NULL OR m.media_type = sqlc.narg('media_type'))
  AND (sqlc.narg('zone_id')::varchar IS NULL OR z.id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR m.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR m.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('event_id')::varchar IS NULL OR m.event_id = sqlc.narg('event_id'))
  AND (sqlc.narg('year')::int IS NULL OR EXTRACT(YEAR FROM e.start_time) = sqlc.narg('year'))
ORDER BY m.is_featured DESC, m.published_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPublicGalleryMedia :one
SELECT COUNT(*)
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN states s ON m.state_id = s.id
JOIN zones z ON s.zone_id = z.id
WHERE m.is_published = TRUE 
  AND m.is_hidden = FALSE 
  AND m.moderation_status = 'approved'
  AND (sqlc.narg('media_type')::varchar IS NULL OR m.media_type = sqlc.narg('media_type'))
  AND (sqlc.narg('zone_id')::varchar IS NULL OR z.id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR m.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR m.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('event_id')::varchar IS NULL OR m.event_id = sqlc.narg('event_id'))
  AND (sqlc.narg('year')::int IS NULL OR EXTRACT(YEAR FROM e.start_time) = sqlc.narg('year'));

-- name: ListFeaturedGalleryMedia :many
SELECT 
    m.id,
    m.event_id,
    m.media_type,
    m.caption,
    m.description,
    m.public_url,
    m.thumbnail_url,
    m.mime_type,
    m.width,
    m.height,
    m.duration_seconds,
    m.published_at,
    e.title AS event_title,
    s.name AS state_name,
    l.name AS lga_name
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN states s ON m.state_id = s.id
JOIN lgas l ON m.lga_id = l.id
WHERE m.is_published = TRUE 
  AND m.is_hidden = FALSE 
  AND m.moderation_status = 'approved'
ORDER BY m.is_featured DESC, m.published_at DESC
LIMIT $1;

-- name: ListEventGalleryMedia :many
SELECT 
    m.id,
    m.event_id,
    m.media_type,
    m.caption,
    m.public_url,
    m.thumbnail_url,
    m.mime_type,
    m.width,
    m.height,
    m.duration_seconds,
    m.published_at,
    m.allow_public_attribution,
    CASE WHEN m.allow_public_attribution THEN u.display_name ELSE NULL END AS uploader_display_name
FROM event_media m
JOIN users u ON m.uploaded_by_user_id = u.id
WHERE m.event_id = $1 
  AND m.is_published = TRUE 
  AND m.is_hidden = FALSE 
  AND m.moderation_status = 'approved'
ORDER BY m.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListMyEventMedia :many
SELECT 
    m.*,
    e.title AS event_title,
    e.start_time AS event_date,
    s.name AS state_name,
    l.name AS lga_name
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN states s ON m.state_id = s.id
JOIN lgas l ON m.lga_id = l.id
WHERE m.uploaded_by_user_id = $1
ORDER BY m.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMyEventMedia :one
SELECT COUNT(*)
FROM event_media
WHERE uploaded_by_user_id = $1;

-- name: ListAdminEventMedia :many
SELECT 
    m.*,
    u.display_name AS uploader_name,
    u.email AS uploader_email,
    e.title AS event_title,
    e.start_time AS event_date,
    s.name AS state_name,
    l.name AS lga_name,
    z.id AS zone_id,
    z.name AS zone_name,
    rev.display_name AS reviewer_name
FROM event_media m
JOIN event_instances e ON m.event_id = e.id
JOIN users u ON m.uploaded_by_user_id = u.id
JOIN states s ON m.state_id = s.id
JOIN lgas l ON m.lga_id = l.id
JOIN zones z ON s.zone_id = z.id
LEFT JOIN users rev ON m.reviewed_by_user_id = rev.id
WHERE (sqlc.narg('moderation_status')::varchar IS NULL OR m.moderation_status = sqlc.narg('moderation_status'))
  AND (sqlc.narg('zone_id')::varchar IS NULL OR z.id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR m.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR m.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('event_id')::varchar IS NULL OR m.event_id = sqlc.narg('event_id'))
ORDER BY 
    CASE WHEN m.moderation_status = 'pending_review' THEN 0 ELSE 1 END,
    m.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAdminEventMedia :one
SELECT COUNT(*)
FROM event_media m
JOIN states s ON m.state_id = s.id
JOIN zones z ON s.zone_id = z.id
WHERE (sqlc.narg('moderation_status')::varchar IS NULL OR m.moderation_status = sqlc.narg('moderation_status'))
  AND (sqlc.narg('zone_id')::varchar IS NULL OR z.id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR m.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR m.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('event_id')::varchar IS NULL OR m.event_id = sqlc.narg('event_id'));

-- name: GetAdminGalleryOverviewStats :one
SELECT 
    COUNT(*) AS total_media,
    COUNT(*) FILTER (WHERE moderation_status = 'pending_review') AS pending_review_count,
    COUNT(*) FILTER (WHERE moderation_status = 'approved') AS approved_count,
    COUNT(*) FILTER (WHERE moderation_status = 'rejected') AS rejected_count,
    COUNT(*) FILTER (WHERE moderation_status = 'revision_requested') AS revision_requested_count,
    COUNT(*) FILTER (WHERE moderation_status = 'removed') AS removed_count,
    COUNT(*) FILTER (WHERE is_hidden = TRUE) AS hidden_count,
    (SELECT COUNT(*) FROM media_reports WHERE status = 'pending') AS pending_reports_count,
    (SELECT COUNT(*) FROM media_takedown_requests WHERE status = 'pending') AS pending_takedowns_count
FROM event_media;

-- name: CreateMediaModerationAction :one
INSERT INTO media_moderation_actions (
    id,
    media_id,
    reviewer_user_id,
    action,
    reason,
    notes
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListMediaModerationActionsByMediaID :many
SELECT 
    a.*,
    u.display_name AS reviewer_name
FROM media_moderation_actions a
JOIN users u ON a.reviewer_user_id = u.id
WHERE a.media_id = $1
ORDER BY a.created_at DESC;

-- name: CreateMediaReport :one
INSERT INTO media_reports (
    id,
    media_id,
    reporter_user_id,
    reporter_email,
    report_reason,
    details,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetMediaReportByID :one
SELECT 
    r.*,
    m.caption AS media_caption,
    m.public_url AS media_url,
    u.display_name AS reporter_name,
    rev.display_name AS resolver_name
FROM media_reports r
JOIN event_media m ON r.media_id = m.id
LEFT JOIN users u ON r.reporter_user_id = u.id
LEFT JOIN users rev ON r.resolved_by_user_id = rev.id
WHERE r.id = $1;

-- name: ListMediaReports :many
SELECT 
    r.*,
    m.caption AS media_caption,
    m.public_url AS media_url,
    m.thumbnail_url AS media_thumbnail,
    u.display_name AS reporter_name,
    rev.display_name AS resolver_name
FROM media_reports r
JOIN event_media m ON r.media_id = m.id
LEFT JOIN users u ON r.reporter_user_id = u.id
LEFT JOIN users rev ON r.resolved_by_user_id = rev.id
WHERE (sqlc.narg('status')::varchar IS NULL OR r.status = sqlc.narg('status'))
ORDER BY 
    CASE WHEN r.status = 'pending' THEN 0 ELSE 1 END,
    r.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountMediaReports :one
SELECT COUNT(*)
FROM media_reports
WHERE (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'));

-- name: ResolveMediaReport :one
UPDATE media_reports
SET status = $2,
    resolution_notes = $3,
    resolved_by_user_id = $4,
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateMediaTakedownRequest :one
INSERT INTO media_takedown_requests (
    id,
    media_id,
    requester_user_id,
    requester_name,
    requester_email,
    relationship_to_media,
    reason,
    details,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetMediaTakedownRequestByID :one
SELECT 
    t.*,
    m.caption AS media_caption,
    m.public_url AS media_url,
    m.thumbnail_url AS media_thumbnail,
    rev.display_name AS reviewer_name
FROM media_takedown_requests t
JOIN event_media m ON t.media_id = m.id
LEFT JOIN users rev ON t.reviewed_by_user_id = rev.id
WHERE t.id = $1;

-- name: ListMediaTakedownRequests :many
SELECT 
    t.*,
    m.caption AS media_caption,
    m.public_url AS media_url,
    m.thumbnail_url AS media_thumbnail,
    rev.display_name AS reviewer_name
FROM media_takedown_requests t
JOIN event_media m ON t.media_id = m.id
LEFT JOIN users rev ON t.reviewed_by_user_id = rev.id
WHERE (sqlc.narg('status')::varchar IS NULL OR t.status = sqlc.narg('status'))
ORDER BY 
    CASE WHEN t.status = 'pending' THEN 0 ELSE 1 END,
    t.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountMediaTakedownRequests :one
SELECT COUNT(*)
FROM media_takedown_requests
WHERE (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'));

-- name: ResolveMediaTakedownRequest :one
UPDATE media_takedown_requests
SET status = $2,
    resolution_reason = $3,
    reviewed_by_user_id = $4,
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;
