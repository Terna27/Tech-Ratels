-- +goose Up
-- Preserve all historical orders and business data; only extend status lifecycle.
ALTER TABLE marketplace_businesses DROP CONSTRAINT IF EXISTS marketplace_businesses_status_check;
ALTER TABLE marketplace_businesses ADD CONSTRAINT marketplace_businesses_status_check
  CHECK (status IN ('pending_review','approved','suspended','rejected','revoked'));

-- +goose Down
-- Revert revoked statuses to suspended before restoring the legacy constraint.
UPDATE marketplace_businesses SET status='suspended',is_verified=false
  WHERE status='revoked';
ALTER TABLE marketplace_businesses DROP CONSTRAINT IF EXISTS marketplace_businesses_status_check;
ALTER TABLE marketplace_businesses ADD CONSTRAINT marketplace_businesses_status_check
  CHECK (status IN ('pending_review','approved','suspended','rejected'));
