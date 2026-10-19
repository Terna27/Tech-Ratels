-- In-App Notifications

-- name: CreateNotification :one
INSERT INTO notifications (
    id, recipient_id, notification_type, title, body,
    related_resource_type, related_resource_id, action_url,
    priority, is_read, read_at, created_at, expires_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11, $12, $13
) RETURNING *;

-- name: ListNotificationsByRecipient :many
SELECT * FROM notifications
WHERE recipient_id = $1
  AND (sqlc.narg('unread_only')::boolean IS NULL OR (sqlc.narg('unread_only')::boolean = true AND is_read = false) OR (sqlc.narg('unread_only')::boolean = false AND is_read = true))
  AND (sqlc.narg('notification_type')::text IS NULL OR notification_type = sqlc.narg('notification_type'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountNotificationsByRecipient :one
SELECT COUNT(*) FROM notifications
WHERE recipient_id = $1
  AND (sqlc.narg('unread_only')::boolean IS NULL OR (sqlc.narg('unread_only')::boolean = true AND is_read = false) OR (sqlc.narg('unread_only')::boolean = false AND is_read = true))
  AND (sqlc.narg('notification_type')::text IS NULL OR notification_type = sqlc.narg('notification_type'));

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE recipient_id = $1 AND is_read = false;

-- name: MarkNotificationRead :one
UPDATE notifications
SET is_read = true,
    read_at = CURRENT_TIMESTAMP
WHERE id = $1 AND recipient_id = $2
RETURNING *;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET is_read = true,
    read_at = CURRENT_TIMESTAMP
WHERE recipient_id = $1 AND is_read = false;

-- name: GetNotificationByID :one
SELECT * FROM notifications
WHERE id = $1 AND recipient_id = $2;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE id = $1 AND recipient_id = $2;

-- Transactional Outbox

-- name: EnqueueOutbox :one
INSERT INTO notification_outbox (
    id, event_type, recipient_id, recipient_email, recipient_phone,
    recipient_name, channel, payload, delivery_state, attempt_count,
    max_attempts, next_attempt_at, idempotency_key, created_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14
) RETURNING *;

-- name: ClaimOutboxBatch :many
UPDATE notification_outbox
SET delivery_state = 'processing',
    lease_token = $1,
    leased_until = $2
WHERE id IN (
    SELECT sub.id FROM notification_outbox sub
    WHERE sub.delivery_state = 'pending'
      AND sub.next_attempt_at <= $3
      AND (sub.leased_until IS NULL OR sub.leased_until < $3)
    ORDER BY sub.next_attempt_at ASC
    LIMIT $4
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: GetOutboxByID :one
SELECT * FROM notification_outbox
WHERE id = $1;

-- name: MarkOutboxDelivered :one
UPDATE notification_outbox
SET delivery_state = 'delivered',
    delivered_at = CURRENT_TIMESTAMP,
    lease_token = NULL,
    leased_until = NULL
WHERE id = $1
RETURNING *;

-- name: MarkOutboxRetry :one
UPDATE notification_outbox
SET delivery_state = 'pending',
    attempt_count = attempt_count + 1,
    next_attempt_at = $2,
    last_error = $3,
    last_error_category = $4,
    lease_token = NULL,
    leased_until = NULL
WHERE id = $1
RETURNING *;

-- name: MarkOutboxDeadLetter :one
UPDATE notification_outbox
SET delivery_state = 'dead_letter',
    attempt_count = attempt_count + 1,
    last_error = $2,
    last_error_category = $3,
    lease_token = NULL,
    leased_until = NULL
WHERE id = $1
RETURNING *;

-- name: MarkOutboxSkipped :one
UPDATE notification_outbox
SET delivery_state = 'skipped',
    last_error = $2,
    last_error_category = 'opt_out',
    lease_token = NULL,
    leased_until = NULL
WHERE id = $1
RETURNING *;

-- name: RetryOutboxItem :one
UPDATE notification_outbox
SET delivery_state = 'pending',
    next_attempt_at = CURRENT_TIMESTAMP,
    attempt_count = 0,
    last_error = NULL,
    last_error_category = NULL,
    lease_token = NULL,
    leased_until = NULL
WHERE id = $1
RETURNING *;

-- name: ListOutboxAdmin :many
SELECT * FROM notification_outbox
WHERE (sqlc.narg('delivery_state')::text IS NULL OR delivery_state = sqlc.narg('delivery_state'))
  AND (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountOutboxAdmin :one
SELECT COUNT(*) FROM notification_outbox
WHERE (sqlc.narg('delivery_state')::text IS NULL OR delivery_state = sqlc.narg('delivery_state'))
  AND (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel'));

-- name: ListFailedOutboxAdmin :many
SELECT * FROM notification_outbox
WHERE delivery_state IN ('failed', 'dead_letter')
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountFailedOutboxAdmin :one
SELECT COUNT(*) FROM notification_outbox
WHERE delivery_state IN ('failed', 'dead_letter');

-- name: GetOutboxOverviewStats :one
SELECT
    COUNT(*) AS total_count,
    COUNT(*) FILTER (WHERE delivery_state = 'pending') AS pending_count,
    COUNT(*) FILTER (WHERE delivery_state = 'processing') AS processing_count,
    COUNT(*) FILTER (WHERE delivery_state = 'delivered') AS delivered_count,
    COUNT(*) FILTER (WHERE delivery_state IN ('failed', 'dead_letter')) AS failed_count,
    COUNT(*) FILTER (WHERE delivery_state = 'skipped') AS skipped_count
FROM notification_outbox;

-- Deliveries Tracking

-- name: CreateDeliveryRecord :one
INSERT INTO notification_deliveries (
    id, outbox_id, provider, channel, provider_message_ref,
    status, failure_category, error_message, attempt_number,
    response_payload, created_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11
) RETURNING *;

-- name: ListDeliveriesAdmin :many
SELECT * FROM notification_deliveries
WHERE (sqlc.narg('provider')::text IS NULL OR provider = sqlc.narg('provider'))
  AND (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountDeliveriesAdmin :one
SELECT COUNT(*) FROM notification_deliveries
WHERE (sqlc.narg('provider')::text IS NULL OR provider = sqlc.narg('provider'))
  AND (sqlc.narg('channel')::text IS NULL OR channel = sqlc.narg('channel'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'));

-- Preferences

-- name: GetNotificationPreferences :one
SELECT * FROM notification_preferences
WHERE user_id = $1;

-- name: UpsertNotificationPreferences :one
INSERT INTO notification_preferences (
    user_id, in_app_enabled, email_enabled, sms_enabled,
    cleanup_reminders_enabled, leadership_alerts_enabled,
    participation_updates_enabled, marketplace_updates_enabled,
    careers_updates_enabled, community_project_updates_enabled,
    finance_alerts_enabled, announcements_enabled,
    sms_consent_recorded_at, sms_consent_withdrawn_at, updated_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6,
    $7, $8,
    $9, $10,
    $11, $12,
    $13, $14, CURRENT_TIMESTAMP
)
ON CONFLICT (user_id) DO UPDATE SET
    in_app_enabled = EXCLUDED.in_app_enabled,
    email_enabled = EXCLUDED.email_enabled,
    sms_enabled = EXCLUDED.sms_enabled,
    cleanup_reminders_enabled = EXCLUDED.cleanup_reminders_enabled,
    leadership_alerts_enabled = EXCLUDED.leadership_alerts_enabled,
    participation_updates_enabled = EXCLUDED.participation_updates_enabled,
    marketplace_updates_enabled = EXCLUDED.marketplace_updates_enabled,
    careers_updates_enabled = EXCLUDED.careers_updates_enabled,
    community_project_updates_enabled = EXCLUDED.community_project_updates_enabled,
    finance_alerts_enabled = EXCLUDED.finance_alerts_enabled,
    announcements_enabled = EXCLUDED.announcements_enabled,
    sms_consent_recorded_at = COALESCE(EXCLUDED.sms_consent_recorded_at, notification_preferences.sms_consent_recorded_at),
    sms_consent_withdrawn_at = COALESCE(EXCLUDED.sms_consent_withdrawn_at, notification_preferences.sms_consent_withdrawn_at),
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- Templates

-- name: GetNotificationTemplateByCode :one
SELECT * FROM notification_templates
WHERE template_code = $1 AND is_active = true;

-- name: ListNotificationTemplates :many
SELECT * FROM notification_templates
ORDER BY template_code ASC;

-- name: UpdateNotificationTemplate :one
UPDATE notification_templates
SET title_template = $2,
    body_template = $3,
    version = version + 1,
    is_active = $4,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- Notification Schedules

-- name: CreateNotificationSchedule :one
INSERT INTO notification_schedules (
    id, schedule_type, event_id, event_schedule_version,
    target_time, status, created_at, updated_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8
)
ON CONFLICT (schedule_type, event_id, event_schedule_version) DO NOTHING
RETURNING *;

-- name: GetDueNotificationSchedules :many
SELECT * FROM notification_schedules
WHERE status = 'pending' AND target_time <= $1
ORDER BY target_time ASC
LIMIT $2;

-- name: MarkScheduleDispatched :one
UPDATE notification_schedules
SET status = 'dispatched',
    dispatched_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CancelPendingSchedulesForEvent :exec
UPDATE notification_schedules
SET status = 'superseded',
    updated_at = CURRENT_TIMESTAMP
WHERE event_id = $1 AND status = 'pending';

-- name: ListNotificationSchedulesAdmin :many
SELECT s.*, e.title as event_title, e.start_time as event_start_time
FROM notification_schedules s
JOIN event_instances e ON e.id = s.event_id
WHERE (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status'))
ORDER BY s.target_time DESC
LIMIT $1 OFFSET $2;

-- name: CountNotificationSchedulesAdmin :one
SELECT COUNT(*)
FROM notification_schedules s
WHERE (sqlc.narg('status')::text IS NULL OR s.status = sqlc.narg('status'));

-- name: ListRegisteredUsersForEvent :many
SELECT u.id, u.email, u.phone, u.display_name
FROM event_registrations r
JOIN users u ON r.user_id = u.id
WHERE r.event_id = $1 AND r.status = 'registered';

-- name: GetEventNotificationContext :one
SELECT e.id, e.title, e.start_time, e.end_time, e.address, e.state_id, s.name as state_name, e.coordinator_id
FROM event_instances e
JOIN states s ON e.state_id = s.id
WHERE e.id = $1;

