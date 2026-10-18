-- -------------------------------------------------------------
-- Community Requests
-- -------------------------------------------------------------

-- name: CreateCommunityRequest :one
INSERT INTO community_requests (
    id, reference, title, description, category,
    state_id, lga_id, community_ward, location_details,
    estimated_urgency, evidence_image_urls, requester_id,
    status, escalation_level, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12,
    $13, $14, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetCommunityRequestByID :one
SELECT r.*,
       s.name as state_name,
       l.name as lga_name,
       u.display_name as requester_name,
       u.email as requester_email,
       u.phone as requester_phone,
       u.membership_reference as requester_membership_reference,
       rv.display_name as reviewer_name
FROM community_requests r
JOIN states s ON r.state_id = s.id
JOIN lgas l ON r.lga_id = l.id
JOIN users u ON r.requester_id = u.id
LEFT JOIN users rv ON r.reviewed_by_user_id = rv.id
WHERE r.id = $1;

-- name: GetCommunityRequestByReference :one
SELECT r.*,
       s.name as state_name,
       l.name as lga_name,
       u.display_name as requester_name,
       u.email as requester_email,
       u.phone as requester_phone,
       u.membership_reference as requester_membership_reference,
       rv.display_name as reviewer_name
FROM community_requests r
JOIN states s ON r.state_id = s.id
JOIN lgas l ON r.lga_id = l.id
JOIN users u ON r.requester_id = u.id
LEFT JOIN users rv ON r.reviewed_by_user_id = rv.id
WHERE r.reference = $1;

-- name: ListCommunityRequestsByRequester :many
SELECT r.*,
       s.name as state_name,
       l.name as lga_name
FROM community_requests r
JOIN states s ON r.state_id = s.id
JOIN lgas l ON r.lga_id = l.id
WHERE r.requester_id = $1
ORDER BY r.created_at DESC;

-- name: ListAdminCommunityRequests :many
SELECT r.*,
       s.name as state_name,
       l.name as lga_name,
       u.display_name as requester_name,
       u.email as requester_email,
       u.phone as requester_phone,
       u.membership_reference as requester_membership_reference,
       rv.display_name as reviewer_name
FROM community_requests r
JOIN states s ON r.state_id = s.id
JOIN lgas l ON r.lga_id = l.id
JOIN users u ON r.requester_id = u.id
LEFT JOIN users rv ON r.reviewed_by_user_id = rv.id
WHERE (sqlc.narg('state_id')::text IS NULL OR r.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::text IS NULL OR r.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('category')::text IS NULL OR r.category = sqlc.narg('category'))
  AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'))
  AND (sqlc.narg('urgency')::text IS NULL OR r.estimated_urgency = sqlc.narg('urgency'))
ORDER BY r.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAdminCommunityRequests :one
SELECT COUNT(*)
FROM community_requests r
WHERE (sqlc.narg('state_id')::text IS NULL OR r.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::text IS NULL OR r.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('category')::text IS NULL OR r.category = sqlc.narg('category'))
  AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'))
  AND (sqlc.narg('urgency')::text IS NULL OR r.estimated_urgency = sqlc.narg('urgency'));

-- name: ReviewCommunityRequest :one
UPDATE community_requests
SET status = $2,
    reviewed_by_user_id = $3,
    reviewed_at = $4,
    review_notes = $5,
    rejection_reason = $6,
    info_request_notes = $7,
    escalation_level = $8,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: ResubmitCommunityRequest :one
UPDATE community_requests
SET description = $2,
    location_details = $3,
    evidence_image_urls = $4,
    status = 'submitted',
    info_request_notes = NULL,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1 AND requester_id = $5
RETURNING *;

-- name: MarkCommunityRequestConverted :one
UPDATE community_requests
SET converted_to_project_id = $2,
    status = 'approved',
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Community Projects
-- -------------------------------------------------------------

-- name: CreateCommunityProject :one
INSERT INTO community_projects (
    id, reference, title, description, community_request_id,
    category, state_id, lga_id, community_ward, location_details,
    responsible_officer_id, responsible_department,
    start_date, target_completion_date,
    proposed_budget, approved_budget, allocated_funding,
    recorded_expenditure, outstanding_commitments,
    funding_status, execution_status, progress_percentage,
    supporting_documents, is_public, created_by_user_id,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12,
    $13, $14,
    $15, $16, $17,
    $18, $19,
    $20, $21, $22,
    $23, $24, $25,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetCommunityProjectByID :one
SELECT p.*,
       s.name as state_name,
       l.name as lga_name,
       ro.display_name as responsible_officer_name,
       ro.email as responsible_officer_email,
       cb.display_name as created_by_name,
       vb.display_name as verified_by_name,
       cr.reference as request_reference,
       cr.title as request_title
FROM community_projects p
JOIN states s ON p.state_id = s.id
JOIN lgas l ON p.lga_id = l.id
LEFT JOIN users ro ON p.responsible_officer_id = ro.id
LEFT JOIN users cb ON p.created_by_user_id = cb.id
LEFT JOIN users vb ON p.verified_by_user_id = vb.id
LEFT JOIN community_requests cr ON p.community_request_id = cr.id
WHERE p.id = $1;

-- name: GetCommunityProjectByReference :one
SELECT p.*,
       s.name as state_name,
       l.name as lga_name,
       ro.display_name as responsible_officer_name,
       ro.email as responsible_officer_email,
       cb.display_name as created_by_name,
       vb.display_name as verified_by_name,
       cr.reference as request_reference,
       cr.title as request_title
FROM community_projects p
JOIN states s ON p.state_id = s.id
JOIN lgas l ON p.lga_id = l.id
LEFT JOIN users ro ON p.responsible_officer_id = ro.id
LEFT JOIN users cb ON p.created_by_user_id = cb.id
LEFT JOIN users vb ON p.verified_by_user_id = vb.id
LEFT JOIN community_requests cr ON p.community_request_id = cr.id
WHERE p.reference = $1;

-- name: ListPublicCommunityProjects :many
SELECT p.id, p.reference, p.title, p.description, p.category,
       p.state_id, p.lga_id, p.community_ward, p.location_details,
       p.start_date, p.target_completion_date, p.actual_completion_date,
       p.approved_budget, p.recorded_expenditure, p.allocated_funding,
       p.funding_status, p.execution_status, p.progress_percentage,
       p.completion_evidence_urls, p.created_at, p.updated_at,
       s.name as state_name,
       l.name as lga_name,
       ro.display_name as responsible_officer_name
FROM community_projects p
JOIN states s ON p.state_id = s.id
JOIN lgas l ON p.lga_id = l.id
LEFT JOIN users ro ON p.responsible_officer_id = ro.id
WHERE p.is_public = TRUE
  AND (sqlc.narg('state_id')::text IS NULL OR p.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::text IS NULL OR p.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('category')::text IS NULL OR p.category = sqlc.narg('category'))
  AND (sqlc.narg('execution_status')::text IS NULL OR p.execution_status = sqlc.narg('execution_status'))
ORDER BY p.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountPublicCommunityProjects :one
SELECT COUNT(*)
FROM community_projects p
WHERE p.is_public = TRUE
  AND (sqlc.narg('state_id')::text IS NULL OR p.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::text IS NULL OR p.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('category')::text IS NULL OR p.category = sqlc.narg('category'))
  AND (sqlc.narg('execution_status')::text IS NULL OR p.execution_status = sqlc.narg('execution_status'));

-- name: ListAdminCommunityProjects :many
SELECT p.*,
       s.name as state_name,
       l.name as lga_name,
       ro.display_name as responsible_officer_name,
       cb.display_name as created_by_name,
       vb.display_name as verified_by_name,
       cr.reference as request_reference
FROM community_projects p
JOIN states s ON p.state_id = s.id
JOIN lgas l ON p.lga_id = l.id
LEFT JOIN users ro ON p.responsible_officer_id = ro.id
LEFT JOIN users cb ON p.created_by_user_id = cb.id
LEFT JOIN users vb ON p.verified_by_user_id = vb.id
LEFT JOIN community_requests cr ON p.community_request_id = cr.id
WHERE (sqlc.narg('state_id')::text IS NULL OR p.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::text IS NULL OR p.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('category')::text IS NULL OR p.category = sqlc.narg('category'))
  AND (sqlc.narg('execution_status')::text IS NULL OR p.execution_status = sqlc.narg('execution_status'))
  AND (sqlc.narg('funding_status')::text IS NULL OR p.funding_status = sqlc.narg('funding_status'))
ORDER BY p.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAdminCommunityProjects :one
SELECT COUNT(*)
FROM community_projects p
WHERE (sqlc.narg('state_id')::text IS NULL OR p.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::text IS NULL OR p.lga_id = sqlc.narg('lga_id'))
  AND (sqlc.narg('category')::text IS NULL OR p.category = sqlc.narg('category'))
  AND (sqlc.narg('execution_status')::text IS NULL OR p.execution_status = sqlc.narg('execution_status'))
  AND (sqlc.narg('funding_status')::text IS NULL OR p.funding_status = sqlc.narg('funding_status'));

-- name: UpdateCommunityProjectDetails :one
UPDATE community_projects
SET title = $2,
    description = $3,
    category = $4,
    state_id = $5,
    lga_id = $6,
    community_ward = $7,
    location_details = $8,
    responsible_officer_id = $9,
    responsible_department = $10,
    start_date = $11,
    target_completion_date = $12,
    is_public = $13,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateCommunityProjectExecution :one
UPDATE community_projects
SET execution_status = $2,
    progress_percentage = $3,
    actual_completion_date = $4,
    pause_or_cancel_reason = $5,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: UpdateCommunityProjectFinances :one
UPDATE community_projects
SET proposed_budget = $2,
    approved_budget = $3,
    allocated_funding = $4,
    recorded_expenditure = $5,
    outstanding_commitments = $6,
    funding_status = $7,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- name: VerifyCommunityProjectCompletion :one
UPDATE community_projects
SET execution_status = 'completed',
    progress_percentage = 100,
    actual_completion_date = $2,
    completion_evidence_urls = $3,
    verified_by_user_id = $4,
    verified_at = $5,
    verification_notes = $6,
    updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE id = $1
RETURNING *;

-- -------------------------------------------------------------
-- Milestones
-- -------------------------------------------------------------

-- name: CreateProjectMilestone :one
INSERT INTO community_project_milestones (
    id, project_id, sequence_order, title, description,
    target_date, completion_date, assigned_person_name, assigned_person_id,
    status, progress_notes, evidence_photo_urls, verification_status,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12, $13,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetProjectMilestoneByID :one
SELECT m.*,
       p.reference as project_reference,
       p.title as project_title,
       u.display_name as assigned_person_user_name,
       v.display_name as verified_by_name
FROM community_project_milestones m
JOIN community_projects p ON m.project_id = p.id
LEFT JOIN users u ON m.assigned_person_id = u.id
LEFT JOIN users v ON m.verified_by_user_id = v.id
WHERE m.id = $1;

-- name: ListMilestonesByProjectID :many
SELECT m.*,
       u.display_name as assigned_person_user_name,
       v.display_name as verified_by_name
FROM community_project_milestones m
LEFT JOIN users u ON m.assigned_person_id = u.id
LEFT JOIN users v ON m.verified_by_user_id = v.id
WHERE m.project_id = $1
ORDER BY m.sequence_order ASC, m.target_date ASC;

-- name: UpdateProjectMilestone :one
UPDATE community_project_milestones
SET sequence_order = $2,
    title = $3,
    description = $4,
    target_date = $5,
    completion_date = $6,
    assigned_person_name = $7,
    assigned_person_id = $8,
    status = $9,
    progress_notes = $10,
    evidence_photo_urls = $11,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: VerifyProjectMilestone :one
UPDATE community_project_milestones
SET verification_status = $2,
    verified_by_user_id = $3,
    verified_at = $4,
    verification_notes = $5,
    status = $6,
    completion_date = $7,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteProjectMilestone :exec
DELETE FROM community_project_milestones
WHERE id = $1;

-- name: CountUnverifiedCompletedMilestones :one
SELECT COUNT(*)
FROM community_project_milestones
WHERE project_id = $1
  AND status = 'completed'
  AND verification_status != 'verified';

-- -------------------------------------------------------------
-- Project Updates
-- -------------------------------------------------------------

-- name: CreateProjectUpdate :one
INSERT INTO community_project_updates (
    id, project_id, milestone_id, author_id,
    title, content, photo_urls, progress_percentage,
    is_public, created_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListPublicUpdatesByProjectID :many
SELECT u.*,
       a.display_name as author_name,
       m.title as milestone_title
FROM community_project_updates u
JOIN users a ON u.author_id = a.id
LEFT JOIN community_project_milestones m ON u.milestone_id = m.id
WHERE u.project_id = $1 AND u.is_public = TRUE
ORDER BY u.created_at DESC;

-- name: ListAllUpdatesByProjectID :many
SELECT u.*,
       a.display_name as author_name,
       m.title as milestone_title
FROM community_project_updates u
JOIN users a ON u.author_id = a.id
LEFT JOIN community_project_milestones m ON u.milestone_id = m.id
WHERE u.project_id = $1
ORDER BY u.created_at DESC;

-- -------------------------------------------------------------
-- Overview Statistics
-- -------------------------------------------------------------

-- name: GetCommunityProjectsOverviewStats :one
SELECT
    (SELECT COUNT(*) FROM community_requests) as total_requests,
    (SELECT COUNT(*) FROM community_requests WHERE status IN ('submitted', 'under_review')) as pending_requests,
    (SELECT COUNT(*) FROM community_requests WHERE status = 'approved') as approved_requests,
    (SELECT COUNT(*) FROM community_projects WHERE execution_status IN ('planned', 'in_progress', 'awaiting_verification')) as active_projects,
    (SELECT COUNT(*) FROM community_projects WHERE execution_status = 'completed') as completed_projects,
    COALESCE((SELECT SUM(approved_budget) FROM community_projects), 0)::BIGINT as total_approved_budget,
    COALESCE((SELECT SUM(allocated_funding) FROM community_projects), 0)::BIGINT as total_allocated_funding,
    COALESCE((SELECT SUM(recorded_expenditure) FROM community_projects), 0)::BIGINT as total_recorded_expenditure;
