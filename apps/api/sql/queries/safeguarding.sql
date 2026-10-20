-- Safeguarding Officers
-- name: GetSafeguardingOfficerByUserID :one
SELECT * FROM safeguarding_officers
WHERE user_id = $1 AND status = 'active'
LIMIT 1;

-- name: ListActiveSafeguardingOfficers :many
SELECT so.*, u.display_name, u.email, u.phone
FROM safeguarding_officers so
JOIN users u ON u.id = so.user_id
WHERE so.status = 'active'
ORDER BY so.is_lead DESC, u.display_name ASC;

-- name: UpsertSafeguardingOfficer :one
INSERT INTO safeguarding_officers (
    id, user_id, role_title, is_lead, can_triage, can_assign, can_investigate, can_escalate, can_close, can_view_audit, status, appointed_by, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP
)
ON CONFLICT (user_id) DO UPDATE SET
    role_title = EXCLUDED.role_title,
    is_lead = EXCLUDED.is_lead,
    can_triage = EXCLUDED.can_triage,
    can_assign = EXCLUDED.can_assign,
    can_investigate = EXCLUDED.can_investigate,
    can_escalate = EXCLUDED.can_escalate,
    can_close = EXCLUDED.can_close,
    can_view_audit = EXCLUDED.can_view_audit,
    status = EXCLUDED.status,
    appointed_by = EXCLUDED.appointed_by,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- Safeguarding Cases
-- name: CreateSafeguardingCase :one
INSERT INTO safeguarding_cases (
    id, public_reference, category, encrypted_description, incident_date,
    state_id, lga_id, related_activity, person_involved,
    is_immediate_danger, is_minor_involved, risk_level, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: GetSafeguardingCaseByID :one
SELECT sc.*, s.name AS state_name, l.name AS lga_name
FROM safeguarding_cases sc
LEFT JOIN states s ON s.id = sc.state_id
LEFT JOIN lgas l ON l.id = sc.lga_id
WHERE sc.id = $1
LIMIT 1;

-- name: GetSafeguardingCaseByPublicRef :one
SELECT sc.*, s.name AS state_name, l.name AS lga_name
FROM safeguarding_cases sc
LEFT JOIN states s ON s.id = sc.state_id
LEFT JOIN lgas l ON l.id = sc.lga_id
WHERE sc.public_reference = $1
LIMIT 1;

-- name: ListSafeguardingCases :many
SELECT sc.*, s.name AS state_name, l.name AS lga_name,
       sr.is_anonymous,
       sa.officer_id AS assigned_officer_id,
       u.display_name AS assigned_officer_name
FROM safeguarding_cases sc
LEFT JOIN states s ON s.id = sc.state_id
LEFT JOIN lgas l ON l.id = sc.lga_id
LEFT JOIN safeguarding_reporters sr ON sr.case_id = sc.id
LEFT JOIN safeguarding_assignments sa ON sa.case_id = sc.id AND sa.status = 'active'
LEFT JOIN users u ON u.id = sa.officer_id
WHERE (sqlc.narg('status')::text IS NULL OR sc.status = sqlc.narg('status'))
  AND (sqlc.narg('risk_level')::text IS NULL OR sc.risk_level = sqlc.narg('risk_level'))
  AND (sqlc.narg('category')::text IS NULL OR sc.category = sqlc.narg('category'))
  AND (sqlc.narg('is_minor_involved')::boolean IS NULL OR sc.is_minor_involved = sqlc.narg('is_minor_involved'))
  AND (sqlc.narg('is_immediate_danger')::boolean IS NULL OR sc.is_immediate_danger = sqlc.narg('is_immediate_danger'))
ORDER BY
  CASE WHEN sc.is_immediate_danger THEN 0 ELSE 1 END ASC,
  CASE sc.risk_level
    WHEN 'critical' THEN 0
    WHEN 'high' THEN 1
    WHEN 'normal' THEN 2
    ELSE 3
  END ASC,
  sc.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSafeguardingCases :one
SELECT COUNT(*) FROM safeguarding_cases sc
WHERE (sqlc.narg('status')::text IS NULL OR sc.status = sqlc.narg('status'))
  AND (sqlc.narg('risk_level')::text IS NULL OR sc.risk_level = sqlc.narg('risk_level'))
  AND (sqlc.narg('category')::text IS NULL OR sc.category = sqlc.narg('category'))
  AND (sqlc.narg('is_minor_involved')::boolean IS NULL OR sc.is_minor_involved = sqlc.narg('is_minor_involved'))
  AND (sqlc.narg('is_immediate_danger')::boolean IS NULL OR sc.is_immediate_danger = sqlc.narg('is_immediate_danger'));

-- name: UpdateSafeguardingCaseTriage :one
UPDATE safeguarding_cases
SET risk_level = $2,
    status = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateSafeguardingCaseStatus :one
UPDATE safeguarding_cases
SET status = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: EscalateSafeguardingCase :one
UPDATE safeguarding_cases
SET is_escalated = true,
    escalated_to = $2,
    escalation_reason = $3,
    status = 'escalated',
    risk_level = 'critical',
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: ResolveSafeguardingCase :one
UPDATE safeguarding_cases
SET status = 'resolved',
    resolution_summary = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CloseSafeguardingCase :one
UPDATE safeguarding_cases
SET status = 'closed',
    closed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- Safeguarding Reporters
-- name: CreateSafeguardingReporter :one
INSERT INTO safeguarding_reporters (
    id, case_id, is_anonymous, reporter_user_id,
    encrypted_name, encrypted_email, encrypted_phone,
    contact_preference, follow_up_secret_hash
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: GetSafeguardingReporterByCaseID :one
SELECT * FROM safeguarding_reporters
WHERE case_id = $1
LIMIT 1;

-- Safeguarding Evidence
-- name: CreateSafeguardingEvidence :one
INSERT INTO safeguarding_evidence (
    id, case_id, uploader_type, uploader_id, storage_key,
    file_name, mime_type, file_size_bytes, sha256_hash,
    is_quarantined, retention_classification
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ListSafeguardingEvidenceByCaseID :many
SELECT * FROM safeguarding_evidence
WHERE case_id = $1
ORDER BY created_at ASC;

-- name: GetSafeguardingEvidenceByID :one
SELECT * FROM safeguarding_evidence
WHERE id = $1
LIMIT 1;

-- name: DeleteSafeguardingEvidence :exec
DELETE FROM safeguarding_evidence
WHERE id = $1;

-- Safeguarding Assignments
-- name: CreateSafeguardingAssignment :one
INSERT INTO safeguarding_assignments (
    id, case_id, officer_id, assigned_by, status, assignment_reason
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetActiveAssignmentByCaseID :one
SELECT sa.*, u.display_name AS officer_name, u.email AS officer_email
FROM safeguarding_assignments sa
JOIN users u ON u.id = sa.officer_id
WHERE sa.case_id = $1 AND sa.status = 'active'
LIMIT 1;

-- name: ListAssignmentsByCaseID :many
SELECT sa.*, u.display_name AS officer_name, u.email AS officer_email
FROM safeguarding_assignments sa
JOIN users u ON u.id = sa.officer_id
WHERE sa.case_id = $1
ORDER BY sa.created_at DESC;

-- name: DeactivateAssignmentsForCase :exec
UPDATE safeguarding_assignments
SET status = 'reassigned', updated_at = CURRENT_TIMESTAMP
WHERE case_id = $1 AND status = 'active';

-- Safeguarding Conflicts
-- name: CreateSafeguardingConflict :one
INSERT INTO safeguarding_conflicts (
    id, case_id, officer_id, conflict_type, declared_by, reason
) VALUES (
    $1, $2, $3, $4, $5, $6
)
ON CONFLICT (case_id, officer_id) DO UPDATE SET
    conflict_type = EXCLUDED.conflict_type,
    declared_by = EXCLUDED.declared_by,
    reason = EXCLUDED.reason,
    created_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetSafeguardingConflict :one
SELECT * FROM safeguarding_conflicts
WHERE case_id = $1 AND officer_id = $2
LIMIT 1;

-- name: ListSafeguardingConflictsByCaseID :many
SELECT sc.*, u.display_name AS officer_name
FROM safeguarding_conflicts sc
JOIN users u ON u.id = sc.officer_id
WHERE sc.case_id = $1
ORDER BY sc.created_at ASC;

-- Safeguarding Messages
-- name: CreateSafeguardingMessage :one
INSERT INTO safeguarding_messages (
    id, case_id, author_type, author_id, visibility, content
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListReporterVisibleMessagesByCaseID :many
SELECT sm.id, sm.case_id, sm.author_type, sm.visibility, sm.content, sm.created_at
FROM safeguarding_messages sm
WHERE sm.case_id = $1 AND sm.visibility = 'reporter'
ORDER BY sm.created_at ASC;

-- name: ListAllMessagesByCaseID :many
SELECT sm.*, u.display_name AS author_name
FROM safeguarding_messages sm
LEFT JOIN users u ON u.id = sm.author_id
WHERE sm.case_id = $1
ORDER BY sm.created_at ASC;

-- Safeguarding Case Actions
-- name: CreateSafeguardingCaseAction :one
INSERT INTO safeguarding_case_actions (
    id, case_id, actor_id, action_type, previous_status, new_status, reason
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: ListSafeguardingCaseActionsByCaseID :many
SELECT sca.*, u.display_name AS actor_name
FROM safeguarding_case_actions sca
LEFT JOIN users u ON u.id = sca.actor_id
WHERE sca.case_id = $1
ORDER BY sca.created_at ASC;

-- Safeguarding Access Logs
-- name: CreateSafeguardingAccessLog :one
INSERT INTO safeguarding_access_logs (
    id, case_id, actor_id, access_type, justification, ip_address
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: ListSafeguardingAccessLogs :many
SELECT sal.*, u.display_name AS actor_name, sc.public_reference
FROM safeguarding_access_logs sal
JOIN users u ON u.id = sal.actor_id
JOIN safeguarding_cases sc ON sc.id = sal.case_id
ORDER BY sal.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountSafeguardingAccessLogs :one
SELECT COUNT(*) FROM safeguarding_access_logs;

-- Safeguarding Retention Records
-- name: CreateSafeguardingRetentionRecord :one
INSERT INTO safeguarding_retention_records (
    id, case_id, retention_category, review_date, is_legal_hold, legal_hold_reason
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetSafeguardingRetentionRecordByCaseID :one
SELECT * FROM safeguarding_retention_records
WHERE case_id = $1
LIMIT 1;

-- name: UpdateSafeguardingLegalHold :one
UPDATE safeguarding_retention_records
SET is_legal_hold = $2,
    legal_hold_reason = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE case_id = $1
RETURNING *;

-- Safeguarding Settings
-- name: GetSafeguardingSetting :one
SELECT * FROM safeguarding_settings
WHERE key = $1
LIMIT 1;

-- name: ListSafeguardingSettings :many
SELECT * FROM safeguarding_settings
ORDER BY key ASC;

-- name: UpsertSafeguardingSetting :one
INSERT INTO safeguarding_settings (
    key, value, description, updated_by, updated_at
) VALUES (
    $1, $2, $3, $4, CURRENT_TIMESTAMP
)
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    description = COALESCE(EXCLUDED.description, safeguarding_settings.description),
    updated_by = EXCLUDED.updated_by,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;
