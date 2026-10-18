-- name: CreateCleanupCampaign :one
INSERT INTO cleanup_campaigns (
    id, year, month, scheduled_date, title, description, status,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetCleanupCampaignByYearMonth :one
SELECT * FROM cleanup_campaigns
WHERE year = $1 AND month = $2;

-- name: GetCleanupCampaignByID :one
SELECT * FROM cleanup_campaigns
WHERE id = $1;

-- name: ListCleanupCampaigns :many
SELECT * FROM cleanup_campaigns
ORDER BY year DESC, month DESC;

-- name: CreateEventInstance :one
INSERT INTO event_instances (
    id, campaign_id, programme_type, title, state_id, lga_id, address, landmark,
    latitude, longitude, start_time, end_time, capacity, registered_count,
    coordinator_id, public_contact, accessibility_notes, safety_notes,
    status, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13, 0,
    $14, $15, $16, $17,
    $18, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetEventInstanceByID :one
SELECT * FROM event_instances
WHERE id = $1;

-- name: GetEventInstanceForUpdate :one
SELECT * FROM event_instances
WHERE id = $1 FOR UPDATE;

-- name: ListPublishedEvents :many
SELECT e.*, s.name as state_name, l.name as lga_name, u.display_name as coordinator_name
FROM event_instances e
JOIN states s ON e.state_id = s.id
JOIN lgas l ON e.lga_id = l.id
JOIN users u ON e.coordinator_id = u.id
WHERE e.status = 'published'
ORDER BY e.start_time ASC;

-- name: ListPublishedEventsByState :many
SELECT e.*, s.name as state_name, l.name as lga_name, u.display_name as coordinator_name
FROM event_instances e
JOIN states s ON e.state_id = s.id
JOIN lgas l ON e.lga_id = l.id
JOIN users u ON e.coordinator_id = u.id
WHERE e.status = 'published' AND e.state_id = $1
ORDER BY e.start_time ASC;

-- name: ListAllEvents :many
SELECT e.*, s.name as state_name, l.name as lga_name, u.display_name as coordinator_name
FROM event_instances e
JOIN states s ON e.state_id = s.id
JOIN lgas l ON e.lga_id = l.id
JOIN users u ON e.coordinator_id = u.id
ORDER BY e.start_time DESC;

-- name: ListEventsByState :many
SELECT e.*, s.name as state_name, l.name as lga_name, u.display_name as coordinator_name
FROM event_instances e
JOIN states s ON e.state_id = s.id
JOIN lgas l ON e.lga_id = l.id
JOIN users u ON e.coordinator_id = u.id
WHERE e.state_id = $1
ORDER BY e.start_time DESC;

-- name: UpdateEventStatus :one
UPDATE event_instances
SET status = $2, rejection_reason = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: RescheduleEvent :one
UPDATE event_instances
SET start_time = $2, end_time = $3, rescheduled_from = $4, rescheduled_reason = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: IncrementRegisteredCount :one
UPDATE event_instances
SET registered_count = registered_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DecrementRegisteredCount :one
UPDATE event_instances
SET registered_count = GREATEST(0, registered_count - 1), updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateEventRegistration :one
INSERT INTO event_registrations (
    id, event_id, user_id, status, registered_at, created_at, updated_at, version
) VALUES (
    $1, $2, $3, 'registered', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetEventRegistration :one
SELECT * FROM event_registrations
WHERE event_id = $1 AND user_id = $2;

-- name: UpdateEventRegistrationStatus :one
UPDATE event_registrations
SET status = $3, cancelled_at = $4, updated_at = CURRENT_TIMESTAMP
WHERE event_id = $1 AND user_id = $2
RETURNING *;

-- name: ListRegistrationsByUser :many
SELECT r.*, e.title as event_title, e.start_time as event_start_time, e.address as event_address, s.name as state_name
FROM event_registrations r
JOIN event_instances e ON r.event_id = e.id
JOIN states s ON e.state_id = s.id
WHERE r.user_id = $1
ORDER BY r.registered_at DESC;

-- name: CountPublishedEvents :one
SELECT COUNT(*) FROM event_instances WHERE status = 'published';

-- name: CountTotalRegistrations :one
SELECT COUNT(*) FROM event_registrations WHERE status = 'registered';

-- name: CountDraftEvents :one
SELECT COUNT(*) FROM event_instances WHERE status = 'draft';
