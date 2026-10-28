-- name: CreateAppointment :one
INSERT INTO appointments (
    id, user_id, role_title, role_type, jurisdiction_type, jurisdiction_id,
    reporting_to_appointment_id, start_date, end_date, status, approval_reference,
    public_contact, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetAppointmentByID :one
SELECT * FROM appointments WHERE id = $1;

-- name: ListActiveAppointmentsByUser :many
SELECT * FROM appointments
WHERE user_id = $1 AND status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP)
ORDER BY start_date DESC;

-- name: ListAppointments :many
SELECT a.*,
       u.display_name as user_name,
       u.display_name as user_display_name,
       u.email as user_email,
       u.phone as user_phone,
       u.membership_reference as user_membership_ref,
       CASE
           WHEN a.jurisdiction_type = 'national' THEN 'National (Nigeria)'
           WHEN a.jurisdiction_type = 'zone' THEN COALESCE(z.name, a.jurisdiction_id, '')
           WHEN a.jurisdiction_type = 'state' THEN COALESCE(s.name, a.jurisdiction_id, '')
           WHEN a.jurisdiction_type = 'lga' THEN COALESCE(l.name, a.jurisdiction_id, '')
           ELSE COALESCE(a.jurisdiction_id, '')
       END AS jurisdiction_name
FROM appointments a
JOIN users u ON a.user_id = u.id
LEFT JOIN zones z ON a.jurisdiction_type = 'zone' AND (a.jurisdiction_id = z.id OR a.jurisdiction_id = z.code)
LEFT JOIN states s ON a.jurisdiction_type = 'state' AND (a.jurisdiction_id = s.id OR a.jurisdiction_id = s.code)
LEFT JOIN lgas l ON a.jurisdiction_type = 'lga' AND a.jurisdiction_id = l.id
ORDER BY a.start_date DESC;

-- name: ListPublicLeadershipDirectory :many
SELECT a.id, a.user_id, a.role_title, a.role_type, a.jurisdiction_type, a.jurisdiction_id,
       a.start_date, a.status, a.public_contact,
       u.display_name as user_name,
       u.display_name as user_display_name,
       CASE
           WHEN a.jurisdiction_type = 'national' THEN 'National (Nigeria)'
           WHEN a.jurisdiction_type = 'zone' THEN COALESCE(z.name, a.jurisdiction_id, '')
           WHEN a.jurisdiction_type = 'state' THEN COALESCE(s.name, a.jurisdiction_id, '')
           WHEN a.jurisdiction_type = 'lga' THEN COALESCE(l.name, a.jurisdiction_id, '')
           ELSE COALESCE(a.jurisdiction_id, '')
       END AS jurisdiction_name
FROM appointments a
JOIN users u ON a.user_id = u.id
LEFT JOIN zones z ON a.jurisdiction_type = 'zone' AND (a.jurisdiction_id = z.id OR a.jurisdiction_id = z.code)
LEFT JOIN states s ON a.jurisdiction_type = 'state' AND (a.jurisdiction_id = s.id OR a.jurisdiction_id = s.code)
LEFT JOIN lgas l ON a.jurisdiction_type = 'lga' AND a.jurisdiction_id = l.id
WHERE a.status = 'active'
  AND (a.end_date IS NULL OR a.end_date > CURRENT_TIMESTAMP)
ORDER BY a.role_type, u.display_name;

-- name: UpdateAppointmentStatus :one
UPDATE appointments
SET status = $2, end_date = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateAppointmentScopeAndDates :one
UPDATE appointments
SET jurisdiction_type = $2,
    jurisdiction_id = $3,
    start_date = $4,
    end_date = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: GetActiveAppointmentByJurisdiction :one
SELECT * FROM appointments
WHERE role_type = $1
  AND jurisdiction_type = $2
  AND (jurisdiction_id = $3 OR ($3 IS NULL AND jurisdiction_id IS NULL))
  AND status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP)
LIMIT 1;

-- name: CreateAppointmentHistory :one
INSERT INTO appointment_history (
    id, appointment_id, action, changed_by_user_id, previous_status, new_status, reason,
    previous_user_id, new_user_id, effective_date,
    timestamp, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: ListAppointmentHistory :many
SELECT h.*,
       u.display_name as changed_by_name,
       prev_u.display_name as previous_user_name,
       new_u.display_name as new_user_name
FROM appointment_history h
LEFT JOIN users u ON h.changed_by_user_id = u.id
LEFT JOIN users prev_u ON h.previous_user_id = prev_u.id
LEFT JOIN users new_u ON h.new_user_id = new_u.id
WHERE h.appointment_id = $1
ORDER BY h.timestamp DESC;

-- name: AcquireLeadershipJurisdictionLock :exec
SELECT pg_advisory_xact_lock(hashtext('appt:' || @role_type::text || ':' || @jurisdiction_type::text || ':' || COALESCE(@jurisdiction_id::text, ''))::bigint);

-- name: CountActiveAppointments :one
SELECT COUNT(*) FROM appointments
WHERE status = 'active'
  AND (end_date IS NULL OR end_date > CURRENT_TIMESTAMP);

