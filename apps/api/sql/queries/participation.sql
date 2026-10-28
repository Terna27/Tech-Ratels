-- name: CreateParticipationClaim :one
INSERT INTO participation_claims (
    id, event_id, user_id, description, evidence_notes, status, submitted_at,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetParticipationClaimByID :one
SELECT * FROM participation_claims
WHERE id = $1;

-- name: GetParticipationClaimByEventAndUser :one
SELECT * FROM participation_claims
WHERE event_id = $1 AND user_id = $2;

-- name: ListPendingClaims :many
SELECT c.*, e.title as event_title, e.state_id, s.name as state_name, u.display_name as user_name
FROM participation_claims c
JOIN event_instances e ON c.event_id = e.id
JOIN states s ON e.state_id = s.id
JOIN users u ON c.user_id = u.id
WHERE c.status = 'pending'
ORDER BY c.submitted_at ASC;

-- name: ListPendingClaimsByState :many
SELECT c.*, e.title as event_title, e.state_id, s.name as state_name, u.display_name as user_name
FROM participation_claims c
JOIN event_instances e ON c.event_id = e.id
JOIN states s ON e.state_id = s.id
JOIN users u ON c.user_id = u.id
WHERE c.status = 'pending' AND e.state_id = $1
ORDER BY c.submitted_at ASC;

-- name: ReviewParticipationClaim :one
UPDATE participation_claims
SET status = $2, reviewed_by_user_id = $3, review_decision = $4, review_reason = $5,
    reviewed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: CreateParticipationAppeal :one
INSERT INTO participation_appeals (
    id, participation_id, appeal_reason, status, appealed_at,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetParticipationAppealByID :one
SELECT * FROM participation_appeals
WHERE id = $1;

-- name: ListClaimsByUser :many
SELECT c.*, e.title as event_title, e.start_time as event_start_time, s.name as state_name
FROM participation_claims c
JOIN event_instances e ON c.event_id = e.id
JOIN states s ON e.state_id = s.id
WHERE c.user_id = $1
ORDER BY c.submitted_at DESC;

-- name: CreateBadge :one
INSERT INTO badges (
    id, code, name, description, rule_version, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetBadgeByCode :one
SELECT * FROM badges
WHERE code = $1;

-- name: ListBadges :many
SELECT * FROM badges
ORDER BY name;

-- name: CreateBadgeAward :one
INSERT INTO badge_awards (
    id, user_id, badge_id, qualifying_participation_id, awarded_at, is_public,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, CURRENT_TIMESTAMP, $5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetBadgeAward :one
SELECT * FROM badge_awards
WHERE user_id = $1 AND badge_id = $2;

-- name: ListBadgeAwardsByUser :many
SELECT a.*, b.code as badge_code, b.name as badge_name, b.description as badge_description
FROM badge_awards a
JOIN badges b ON a.badge_id = b.id
WHERE a.user_id = $1
ORDER BY a.awarded_at DESC;

-- name: CountVerifiedParticipationsByUser :one
SELECT COUNT(*) FROM participation_claims
WHERE user_id = $1 AND status = 'verified';

-- name: CountPendingClaims :one
SELECT COUNT(*) FROM participation_claims
WHERE status = 'pending';

-- name: CountPendingAppeals :one
SELECT COUNT(*) FROM participation_appeals
WHERE status = 'pending';

-- name: UpsertVerifiedParticipationClaim :one
INSERT INTO participation_claims (
    id, event_id, user_id, description, evidence_notes, status, submitted_at,
    reviewed_by_user_id, review_decision, review_reason, reviewed_at,
    verification_method, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, 'verified', CURRENT_TIMESTAMP,
    $6, 'approved', $7, CURRENT_TIMESTAMP,
    $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
)
ON CONFLICT (event_id, user_id) DO UPDATE
SET status = 'verified',
    reviewed_by_user_id = EXCLUDED.reviewed_by_user_id,
    review_decision = 'approved',
    review_reason = EXCLUDED.review_reason,
    reviewed_at = CURRENT_TIMESTAMP,
    verification_method = EXCLUDED.verification_method,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: ReverseParticipationClaim :one
UPDATE participation_claims
SET status = 'reversed',
    reversed_by_user_id = $2,
    reversal_reason = $3,
    reversed_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND status IN ('verified', 'approved')
RETURNING *;

-- name: ListPendingClaimsByZone :many
SELECT c.*, e.title as event_title, e.state_id, s.name as state_name, u.display_name as user_name
FROM participation_claims c
JOIN event_instances e ON c.event_id = e.id
JOIN states s ON e.state_id = s.id
JOIN users u ON c.user_id = u.id
WHERE c.status = 'pending' AND (s.zone_id = $1 OR e.zone_id = $1)
ORDER BY c.submitted_at ASC;

-- name: ListPendingClaimsByLGA :many
SELECT c.*, e.title as event_title, e.state_id, s.name as state_name, u.display_name as user_name
FROM participation_claims c
JOIN event_instances e ON c.event_id = e.id
JOIN states s ON e.state_id = s.id
JOIN users u ON c.user_id = u.id
WHERE c.status = 'pending' AND e.lga_id = $1
ORDER BY c.submitted_at ASC;

-- name: SubmitEvidenceCorrection :one
INSERT INTO participation_claims (
    id, event_id, user_id, description, evidence_notes, evidence_url, evidence_type,
    status, submitted_at, verification_method, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    'pending', CURRENT_TIMESTAMP, 'evidence_correction', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
)
ON CONFLICT (event_id, user_id) DO UPDATE
SET description = EXCLUDED.description,
    evidence_notes = EXCLUDED.evidence_notes,
    evidence_url = EXCLUDED.evidence_url,
    evidence_type = EXCLUDED.evidence_type,
    status = 'pending',
    submitted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE participation_claims.status = 'pending'
RETURNING *;
