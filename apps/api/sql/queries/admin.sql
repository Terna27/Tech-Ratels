-- ==============================================================================
-- Super Admin Dashboard & Administrative Oversight Queries
-- ==============================================================================

-- name: CountVerifiedUsers :one
SELECT COUNT(*) FROM users WHERE is_verified = true;

-- name: CountTotalCoordinators :one
SELECT COUNT(DISTINCT user_id)
FROM appointments
WHERE status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP);

-- name: CountActiveCoordinatorsByJurisdictionType :many
SELECT jurisdiction_type, COUNT(DISTINCT user_id)::bigint as coordinator_count
FROM appointments
WHERE status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP)
GROUP BY jurisdiction_type;

-- name: CountCoveredZones :one
SELECT COUNT(DISTINCT jurisdiction_id)
FROM appointments
WHERE status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP)
  AND jurisdiction_type = 'zone';

-- name: CountCoveredStates :one
SELECT COUNT(DISTINCT jurisdiction_id)
FROM appointments
WHERE status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP)
  AND jurisdiction_type = 'state';

-- name: CountCoveredLGAs :one
SELECT COUNT(DISTINCT jurisdiction_id)
FROM appointments
WHERE status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP)
  AND jurisdiction_type = 'lga';

-- name: CountUpcomingCleanupActivities :one
SELECT COUNT(*)
FROM event_instances
WHERE status = 'published'
  AND start_time >= CURRENT_TIMESTAMP;

-- name: ListUpcomingCleanupActivities :many
SELECT e.id, e.title, e.programme_type, e.state_id, s.name as state_name,
       e.lga_id, l.name as lga_name, e.address, e.start_time, e.end_time,
       e.capacity, e.registered_count, e.status, COALESCE(u.display_name, 'Unassigned')::varchar as coordinator_name
FROM event_instances e
JOIN states s ON e.state_id = s.id
JOIN lgas l ON e.lga_id = l.id
LEFT JOIN users u ON e.coordinator_id = u.id
WHERE e.status = 'published'
  AND e.start_time >= CURRENT_TIMESTAMP
ORDER BY e.start_time ASC
LIMIT $1;

-- name: ListRecentAuditLogsWithActors :many
SELECT a.id, a.actor_id, COALESCE(u.display_name, 'System')::varchar as actor_name,
       COALESCE(u.email, '')::varchar as actor_email,
       a.actor_role, a.action, a.resource_type, a.resource_id,
       a.scope_type, a.scope_id, a.details, a.ip_address, a.created_at
FROM audit_events a
LEFT JOIN users u ON a.actor_id = u.id
ORDER BY a.created_at DESC
LIMIT $1;

-- name: ListZoneCoverage :many
SELECT * FROM (
    SELECT DISTINCT ON (z.id)
           z.id as zone_id, z.code as zone_code, z.name as zone_name,
           a.id as appointment_id, a.user_id, u.display_name as coordinator_name,
           u.email as coordinator_email, u.phone as coordinator_phone,
           a.start_date, a.status as appointment_status
    FROM zones z
    LEFT JOIN appointments a ON a.jurisdiction_type = 'zone'
      AND a.jurisdiction_id = z.id
      AND a.status = 'active'
      AND (a.end_date IS NULL OR a.end_date > CURRENT_TIMESTAMP)
    LEFT JOIN users u ON a.user_id = u.id
    ORDER BY z.id, a.start_date DESC NULLS LAST
) zone_cov
ORDER BY zone_cov.zone_code;

-- name: ListStateCoverage :many
SELECT * FROM (
    SELECT DISTINCT ON (s.id)
           s.id as state_id, s.code as state_code, s.name as state_name,
           s.is_fct, z.id as zone_id, z.code as zone_code, z.name as zone_name,
           a.id as appointment_id, a.user_id, u.display_name as coordinator_name,
           u.email as coordinator_email, u.phone as coordinator_phone,
           a.start_date, a.status as appointment_status
    FROM states s
    JOIN zones z ON s.zone_id = z.id
    LEFT JOIN appointments a ON a.jurisdiction_type = 'state'
      AND a.jurisdiction_id = s.id
      AND a.status = 'active'
      AND (a.end_date IS NULL OR a.end_date > CURRENT_TIMESTAMP)
    LEFT JOIN users u ON a.user_id = u.id
    ORDER BY s.id, a.start_date DESC NULLS LAST
) state_cov
ORDER BY state_cov.state_name;

-- name: ListLGACoverageByState :many
SELECT * FROM (
    SELECT DISTINCT ON (l.id)
           l.id as lga_id, l.name as lga_name, l.is_area_council, l.state_id,
           a.id as appointment_id, a.user_id, u.display_name as coordinator_name,
           u.email as coordinator_email, u.phone as coordinator_phone,
           a.start_date, a.status as appointment_status
    FROM lgas l
    LEFT JOIN appointments a ON a.jurisdiction_type = 'lga'
      AND a.jurisdiction_id = l.id
      AND a.status = 'active'
      AND (a.end_date IS NULL OR a.end_date > CURRENT_TIMESTAMP)
    LEFT JOIN users u ON a.user_id = u.id
    WHERE l.state_id = $1
    ORDER BY l.id, a.start_date DESC NULLS LAST
) lga_cov
ORDER BY lga_cov.lga_name;

-- name: ListAllLGACoverage :many
SELECT * FROM (
    SELECT DISTINCT ON (l.id)
           l.id as lga_id, l.name as lga_name, l.is_area_council, l.state_id,
           a.id as appointment_id, a.user_id, u.display_name as coordinator_name,
           u.email as coordinator_email, u.phone as coordinator_phone,
           a.start_date, a.status as appointment_status
    FROM lgas l
    LEFT JOIN appointments a ON a.jurisdiction_type = 'lga'
      AND a.jurisdiction_id = l.id
      AND a.status = 'active'
      AND (a.end_date IS NULL OR a.end_date > CURRENT_TIMESTAMP)
    LEFT JOIN users u ON a.user_id = u.id
    ORDER BY l.id, a.start_date DESC NULLS LAST
) lga_cov
ORDER BY lga_cov.lga_name;

-- name: ListOrganizationSettings :many
SELECT key, value, description, category, is_sensitive, updated_by, updated_at
FROM organization_settings
ORDER BY category, key;

-- name: GetOrganizationSetting :one
SELECT key, value, description, category, is_sensitive, updated_by, updated_at
FROM organization_settings
WHERE key = $1;

-- name: UpdateOrganizationSetting :one
UPDATE organization_settings
SET value = $2,
    updated_by = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE key = $1
RETURNING key, value, description, category, is_sensitive, updated_by, updated_at;

