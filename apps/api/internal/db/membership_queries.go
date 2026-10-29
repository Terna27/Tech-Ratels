package db

import "context"

// A stale email/contact verification must never undo an administrative restriction.
// UPDATE locks the user row and rechecks status after any concurrent status change.
func (q *Queries) ActivateVerifiedMember(ctx context.Context, id string) error {
	var activated string
	return q.db.QueryRow(ctx, `UPDATE users SET status='active',is_verified=true,updated_at=now(),version=version+1 WHERE id=$1 AND status IN ('pending_verification','active') RETURNING id`, id).Scan(&activated)
}
