-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    id, actor_id, actor_role, action, resource_type, resource_id,
    scope_type, scope_id, details, ip_address,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: ListAuditEvents :many
SELECT * FROM audit_events
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAuditEvents :one
SELECT COUNT(*) FROM audit_events;
