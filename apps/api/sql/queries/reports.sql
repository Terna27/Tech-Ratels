-- Public Report Definitions
-- name: ListReportDefinitions :many
SELECT * FROM public_report_definitions
WHERE is_active = TRUE
ORDER BY name ASC;

-- name: GetReportDefinitionByType :one
SELECT * FROM public_report_definitions
WHERE report_type = $1
LIMIT 1;

-- Snapshots
-- name: CreateReportSnapshot :one
INSERT INTO public_report_snapshots (
    id, public_reference, definition_id, report_type, title,
    reporting_period, period_start, period_end,
    scope_type, zone_id, state_id, lga_id,
    status, aggregated_data, data_hash,
    publication_version, correction_notes, created_by_user_id,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8,
    $9, $10, $11, $12,
    $13, $14, $15,
    $16, $17, $18,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: GetReportSnapshotByID :one
SELECT rs.*,
       u.display_name AS creator_name,
       z.name AS zone_name,
       s.name AS state_name,
       l.name AS lga_name
FROM public_report_snapshots rs
JOIN users u ON u.id = rs.created_by_user_id
LEFT JOIN zones z ON z.id = rs.zone_id
LEFT JOIN states s ON s.id = rs.state_id
LEFT JOIN lgas l ON l.id = rs.lga_id
WHERE rs.id = $1
LIMIT 1;

-- name: GetReportSnapshotByRef :one
SELECT rs.*,
       u.display_name AS creator_name,
       z.name AS zone_name,
       s.name AS state_name,
       l.name AS lga_name
FROM public_report_snapshots rs
JOIN users u ON u.id = rs.created_by_user_id
LEFT JOIN zones z ON z.id = rs.zone_id
LEFT JOIN states s ON s.id = rs.state_id
LEFT JOIN lgas l ON l.id = rs.lga_id
WHERE rs.public_reference = $1
LIMIT 1;

-- name: ListReportSnapshotsAdmin :many
SELECT rs.*,
       u.display_name AS creator_name,
       z.name AS zone_name,
       s.name AS state_name,
       l.name AS lga_name
FROM public_report_snapshots rs
JOIN users u ON u.id = rs.created_by_user_id
LEFT JOIN zones z ON z.id = rs.zone_id
LEFT JOIN states s ON s.id = rs.state_id
LEFT JOIN lgas l ON l.id = rs.lga_id
WHERE (sqlc.narg('status')::varchar IS NULL OR rs.status = sqlc.narg('status'))
  AND (sqlc.narg('report_type')::varchar IS NULL OR rs.report_type = sqlc.narg('report_type'))
  AND (sqlc.narg('scope_type')::varchar IS NULL OR rs.scope_type = sqlc.narg('scope_type'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR rs.state_id = sqlc.narg('state_id'))
ORDER BY rs.created_at DESC
LIMIT sqlc.arg('limit_count') OFFSET sqlc.arg('offset_count');

-- name: CountReportSnapshotsAdmin :one
SELECT COUNT(*) FROM public_report_snapshots
WHERE (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('report_type')::varchar IS NULL OR report_type = sqlc.narg('report_type'))
  AND (sqlc.narg('scope_type')::varchar IS NULL OR scope_type = sqlc.narg('scope_type'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR state_id = sqlc.narg('state_id'));

-- name: CheckReportSnapshotExists :one
SELECT COUNT(*)::bigint FROM public_report_snapshots
WHERE report_type = $1
  AND reporting_period = $2
  AND scope_type = $3
  AND (
      (scope_type = 'national' AND zone_id IS NULL AND state_id IS NULL AND lga_id IS NULL)
      OR (scope_type = 'zone' AND zone_id = sqlc.narg('zone_id'))
      OR (scope_type = 'state' AND state_id = sqlc.narg('state_id'))
      OR (scope_type = 'lga' AND lga_id = sqlc.narg('lga_id'))
  )
  AND status NOT IN ('withdrawn', 'superseded');

-- name: GetFirstActiveAdminUser :one
SELECT * FROM users
WHERE primary_role IN ('super_admin', 'national_admin')
  AND status = 'active'
ORDER BY created_at ASC
LIMIT 1;

-- name: UpdateReportSnapshotStatus :one
UPDATE public_report_snapshots
SET status = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateReportSnapshotRevision :one
UPDATE public_report_snapshots
SET status = $2,
    publication_version = $3,
    correction_notes = $4,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- Approvals
-- name: CreateReportApproval :one
INSERT INTO public_report_approvals (
    id, snapshot_id, reviewer_id, decision, decision_reason, governance_scope, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: ListReportApprovalsBySnapshot :many
SELECT ra.*, u.display_name AS reviewer_name, u.email AS reviewer_email
FROM public_report_approvals ra
JOIN users u ON u.id = ra.reviewer_id
WHERE ra.snapshot_id = $1
ORDER BY ra.created_at DESC;

-- Publications
-- name: CreateReportPublication :one
INSERT INTO public_report_publications (
    id, snapshot_id, status, published_by_user_id, published_at, created_at
) VALUES (
    $1, $2, 'published', $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: WithdrawReportPublication :one
UPDATE public_report_publications
SET status = 'withdrawn',
    withdrawn_by_user_id = $2,
    withdrawn_at = CURRENT_TIMESTAMP,
    withdrawal_reason = $3
WHERE snapshot_id = $1
RETURNING *;

-- name: ListPublicPublications :many
SELECT rp.id AS publication_id,
       rp.status AS publication_status,
       rp.published_at,
       rp.withdrawn_at,
       rp.withdrawal_reason,
       rp.correction_notice,
       rs.id AS snapshot_id,
       rs.public_reference,
       rs.report_type,
       rs.title,
       rs.reporting_period,
       rs.period_start,
       rs.period_end,
       rs.scope_type,
       rs.zone_id,
       rs.state_id,
       rs.lga_id,
       rs.data_hash,
       rs.publication_version,
       rs.aggregated_data,
       z.name AS zone_name,
       s.name AS state_name,
       l.name AS lga_name,
       u.display_name AS publisher_name
FROM public_report_publications rp
JOIN public_report_snapshots rs ON rs.id = rp.snapshot_id
JOIN users u ON u.id = rp.published_by_user_id
LEFT JOIN zones z ON z.id = rs.zone_id
LEFT JOIN states s ON s.id = rs.state_id
LEFT JOIN lgas l ON l.id = rs.lga_id
WHERE rp.status = 'published' AND rs.status = 'published'
  AND (sqlc.narg('report_type')::varchar IS NULL OR rs.report_type = sqlc.narg('report_type'))
  AND (sqlc.narg('scope_type')::varchar IS NULL OR rs.scope_type = sqlc.narg('scope_type'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR rs.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('reporting_period')::varchar IS NULL OR rs.reporting_period = sqlc.narg('reporting_period'))
ORDER BY rp.published_at DESC
LIMIT sqlc.arg('limit_count') OFFSET sqlc.arg('offset_count');

-- name: CountPublicPublications :one
SELECT COUNT(*)
FROM public_report_publications rp
JOIN public_report_snapshots rs ON rs.id = rp.snapshot_id
WHERE rp.status = 'published' AND rs.status = 'published'
  AND (sqlc.narg('report_type')::varchar IS NULL OR rs.report_type = sqlc.narg('report_type'))
  AND (sqlc.narg('scope_type')::varchar IS NULL OR rs.scope_type = sqlc.narg('scope_type'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR rs.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('reporting_period')::varchar IS NULL OR rs.reporting_period = sqlc.narg('reporting_period'));

-- name: GetPublishedReportByID :one
SELECT rp.id AS publication_id,
       rp.status AS publication_status,
       rp.published_at,
       rp.withdrawn_at,
       rp.withdrawal_reason,
       rp.correction_notice,
       rs.id AS snapshot_id,
       rs.public_reference,
       rs.report_type,
       rs.title,
       rs.reporting_period,
       rs.period_start,
       rs.period_end,
       rs.scope_type,
       rs.zone_id,
       rs.state_id,
       rs.lga_id,
       rs.data_hash,
       rs.publication_version,
       rs.aggregated_data,
       z.name AS zone_name,
       s.name AS state_name,
       l.name AS lga_name,
       u.display_name AS publisher_name
FROM public_report_publications rp
JOIN public_report_snapshots rs ON rs.id = rp.snapshot_id
JOIN users u ON u.id = rp.published_by_user_id
LEFT JOIN zones z ON z.id = rs.zone_id
LEFT JOIN states s ON s.id = rs.state_id
LEFT JOIN lgas l ON l.id = rs.lga_id
WHERE (rp.id = $1 OR rs.id = $1 OR rs.public_reference = $1)
  AND rp.status = 'published' AND rs.status = 'published'
LIMIT 1;

-- Exports
-- name: UpsertReportExport :one
INSERT INTO public_report_exports (
    id, snapshot_id, export_format, file_name, content_hash, storage_reference, export_status, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP
)
ON CONFLICT (id) DO UPDATE SET
    file_name = EXCLUDED.file_name,
    content_hash = EXCLUDED.content_hash,
    storage_reference = EXCLUDED.storage_reference,
    export_status = EXCLUDED.export_status
RETURNING *;

-- Audit
-- name: CreateReportAudit :one
INSERT INTO public_report_audit (
    id, snapshot_id, action, performed_by_user_id, details, ip_address, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: ListReportAudits :many
SELECT ra.*, u.display_name AS actor_name, u.email AS actor_email
FROM public_report_audit ra
LEFT JOIN users u ON u.id = ra.performed_by_user_id
WHERE (sqlc.narg('snapshot_id')::varchar IS NULL OR ra.snapshot_id = sqlc.narg('snapshot_id'))
ORDER BY ra.created_at DESC
LIMIT sqlc.arg('limit_count') OFFSET sqlc.arg('offset_count');

-- name: AggregateCleanupsMetrics :one
SELECT
    COUNT(DISTINCT e.id)::bigint AS completed_cleanups,
    COUNT(DISTINCT e.state_id)::bigint AS active_states,
    COUNT(DISTINCT e.lga_id)::bigint AS active_lgas,
    COALESCE(SUM(e.waste_collected_kg), 0)::bigint AS total_waste_kg,
    COALESCE(SUM(e.waste_bags_count), 0)::bigint AS total_waste_bags,
    COALESCE(SUM(e.cleanup_duration_hours), 0)::numeric AS total_duration_hours,
    COALESCE(SUM(e.registered_count), 0)::bigint AS total_registrations
FROM event_instances e
JOIN states s ON s.id = e.state_id
WHERE (e.status = 'completed' OR e.status = 'published')
  AND e.start_time >= $1 AND e.start_time <= $2
  AND (sqlc.narg('zone_id')::varchar IS NULL OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR e.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR e.lga_id = sqlc.narg('lga_id'));

-- name: AggregateCleanupsParticipation :one
SELECT
    COUNT(pc.id)::bigint AS verified_attendees,
    COUNT(DISTINCT pc.user_id)::bigint AS unique_participants
FROM participation_claims pc
JOIN event_instances e ON e.id = pc.event_id
JOIN states s ON s.id = e.state_id
WHERE pc.status = 'approved'
  AND e.start_time >= $1 AND e.start_time <= $2
  AND (sqlc.narg('zone_id')::varchar IS NULL OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR e.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR e.lga_id = sqlc.narg('lga_id'));

-- name: ListCleanupsBreakdownByState :many
SELECT
    s.id AS state_id,
    s.name AS state_name,
    COUNT(DISTINCT e.id)::bigint AS cleanups_count,
    COALESCE(SUM(e.waste_collected_kg), 0)::bigint AS waste_kg,
    COUNT(pc.id)::bigint AS verified_attendees
FROM states s
LEFT JOIN event_instances e ON e.state_id = s.id AND (e.status = 'completed' OR e.status = 'published')
  AND e.start_time >= $1 AND e.start_time <= $2
  AND (sqlc.narg('lga_id')::varchar IS NULL OR e.lga_id = sqlc.narg('lga_id'))
LEFT JOIN participation_claims pc ON pc.event_id = e.id AND pc.status = 'approved'
WHERE (sqlc.narg('zone_id')::varchar IS NULL OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR s.id = sqlc.narg('state_id'))
GROUP BY s.id, s.name
ORDER BY cleanups_count DESC, s.name ASC;

-- name: AggregateParticipationMetrics :one
SELECT
    COUNT(pc.id)::bigint AS total_claims,
    COUNT(CASE WHEN pc.status = 'approved' THEN 1 END)::bigint AS approved_claims,
    COUNT(CASE WHEN pc.status = 'rejected' THEN 1 END)::bigint AS rejected_claims,
    COUNT(DISTINCT CASE WHEN pc.status = 'approved' THEN pc.user_id END)::bigint AS unique_verified_members
FROM participation_claims pc
JOIN event_instances e ON e.id = pc.event_id
JOIN states s ON s.id = e.state_id
WHERE e.start_time >= $1 AND e.start_time <= $2
  AND (sqlc.narg('zone_id')::varchar IS NULL OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR e.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR e.lga_id = sqlc.narg('lga_id'));

-- name: AggregateFinanceMetrics :one
SELECT
    COALESCE(SUM(CASE WHEN t.transaction_type = 'income' AND t.verification_status = 'verified' THEN t.amount ELSE 0 END), 0)::bigint AS verified_income_kobo,
    COALESCE(SUM(CASE WHEN t.transaction_type = 'expense' AND t.verification_status = 'verified' THEN t.amount ELSE 0 END), 0)::bigint AS verified_expense_kobo
FROM financial_transactions t
LEFT JOIN states s ON s.id = t.state_id
WHERE t.recorded_at >= $1 AND t.recorded_at <= $2
  AND (sqlc.narg('zone_id')::varchar IS NULL OR t.zone_id = sqlc.narg('zone_id') OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR t.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR t.lga_id = sqlc.narg('lga_id'));

-- name: AggregateExpenseCategories :many
SELECT
    fe.category,
    COUNT(fe.id)::bigint AS expense_count,
    COALESCE(SUM(fe.amount_kobo), 0)::bigint AS total_kobo
FROM financial_expenses fe
LEFT JOIN states s ON s.id = fe.state_id
WHERE fe.status IN ('paid', 'reconciled')
  AND fe.created_at >= $1 AND fe.created_at <= $2
  AND (sqlc.narg('zone_id')::varchar IS NULL OR fe.zone_id = sqlc.narg('zone_id') OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR fe.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR fe.lga_id = sqlc.narg('lga_id'))
GROUP BY fe.category
ORDER BY total_kobo DESC;

-- name: AggregateBudgetsMetrics :one
SELECT
    COUNT(b.id)::bigint AS budgets_count,
    COALESCE(SUM(b.approved_amount_kobo), 0)::bigint AS total_approved_budget_kobo,
    COALESCE(SUM(b.spent_amount_kobo), 0)::bigint AS total_spent_kobo
FROM financial_budgets b
LEFT JOIN states s ON s.id = b.state_id
WHERE b.status IN ('approved', 'active', 'closed')
  AND b.period_start >= sqlc.arg('period_start')::date AND b.period_end <= sqlc.arg('period_end')::date
  AND (sqlc.narg('zone_id')::varchar IS NULL OR b.zone_id = sqlc.narg('zone_id') OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR b.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR b.lga_id = sqlc.narg('lga_id'));

-- name: AggregateFoundationDonations :one
SELECT
    COUNT(fd.id)::bigint AS verified_donations_count,
    COALESCE(SUM(fd.amount_kobo), 0)::bigint AS total_donations_kobo
FROM financial_donations fd
LEFT JOIN states s ON s.id = fd.state_id
WHERE fd.entity_type = 'foundation'
  AND fd.status = 'verified'
  AND fd.created_at >= $1 AND fd.created_at <= $2
  AND (sqlc.narg('zone_id')::varchar IS NULL OR fd.zone_id = sqlc.narg('zone_id') OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR fd.state_id = sqlc.narg('state_id'));

-- name: AggregateCommunityProjectsMetrics :one
SELECT
    COUNT(cp.id)::bigint AS total_projects,
    COUNT(CASE WHEN cp.execution_status = 'completed' THEN 1 END)::bigint AS completed_projects,
    COUNT(CASE WHEN cp.execution_status = 'in_progress' THEN 1 END)::bigint AS in_progress_projects,
    COUNT(CASE WHEN cp.execution_status = 'planned' THEN 1 END)::bigint AS planned_projects,
    COALESCE(SUM(cp.approved_budget), 0)::bigint AS total_approved_budget_kobo,
    COALESCE(SUM(cp.recorded_expenditure), 0)::bigint AS total_recorded_expenditure_kobo
FROM community_projects cp
JOIN states s ON s.id = cp.state_id
WHERE cp.is_public = TRUE
  AND (sqlc.narg('zone_id')::varchar IS NULL OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR cp.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR cp.lga_id = sqlc.narg('lga_id'));

-- name: ListCommunityProjectsPublicReport :many
SELECT
    cp.id,
    cp.reference,
    cp.title,
    cp.description,
    cp.category,
    cp.community_ward,
    cp.location_details,
    cp.start_date,
    cp.target_completion_date,
    cp.actual_completion_date,
    cp.approved_budget,
    cp.recorded_expenditure,
    cp.funding_status,
    cp.execution_status,
    cp.progress_percentage,
    cp.verified_at,
    s.name AS state_name,
    l.name AS lga_name
FROM community_projects cp
JOIN states s ON s.id = cp.state_id
JOIN lgas l ON l.id = cp.lga_id
WHERE cp.is_public = TRUE
  AND (sqlc.narg('zone_id')::varchar IS NULL OR s.zone_id = sqlc.narg('zone_id'))
  AND (sqlc.narg('state_id')::varchar IS NULL OR cp.state_id = sqlc.narg('state_id'))
  AND (sqlc.narg('lga_id')::varchar IS NULL OR cp.lga_id = sqlc.narg('lga_id'))
ORDER BY cp.created_at DESC
LIMIT sqlc.arg('limit_count') OFFSET sqlc.arg('offset_count');


-- name: GetPublishedOverviewByScope :one
SELECT
    rs.aggregated_data
FROM public_report_publications rp
JOIN public_report_snapshots rs
    ON rs.id = rp.snapshot_id
WHERE rp.status = 'published'
  AND rs.status = 'published'
  AND rs.report_type = 'national_overview'
  AND rs.scope_type = sqlc.arg('scope_type')::varchar
  AND rs.reporting_period = sqlc.arg('reporting_period')::varchar
  AND rs.zone_id IS NOT DISTINCT FROM sqlc.narg('zone_id')::varchar
  AND rs.state_id IS NOT DISTINCT FROM sqlc.narg('state_id')::varchar
  AND rs.lga_id IS NOT DISTINCT FROM sqlc.narg('lga_id')::varchar
ORDER BY rp.published_at DESC, rp.id DESC
LIMIT 1;
