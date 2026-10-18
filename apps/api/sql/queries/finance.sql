-- name: GetFinancialFundByID :one
SELECT * FROM financial_funds WHERE id = $1;

-- name: ListFinancialFunds :many
SELECT * FROM financial_funds WHERE is_active = true ORDER BY name;

-- name: CreateFinancialTransaction :one
INSERT INTO financial_transactions (
    id, reference, idempotency_key, transaction_type, status, amount, currency,
    source_fund_id, destination_fund_id, related_campaign_id, related_project_id,
    state_id, zone_id, provider_reference, supporting_evidence_url, notes,
    initiating_actor_id, initiating_actor_type, approved_by_user_id, approved_at,
    is_external_income, occurred_at, recorded_at, reversal_of_transaction_id,
    is_reversal, reversal_reason, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11,
    $12, $13, $14, $15, $16,
    $17, $18, $19, $20,
    $21, $22, CURRENT_TIMESTAMP, $23,
    $24, $25, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetFinancialTransactionByID :one
SELECT t.*,
       sf.name as source_fund_name,
       df.name as destination_fund_name,
       s.name as state_name,
       z.name as zone_name,
       u_init.display_name as initiating_actor_name,
       u_app.display_name as approved_by_name
FROM financial_transactions t
LEFT JOIN financial_funds sf ON t.source_fund_id = sf.id
LEFT JOIN financial_funds df ON t.destination_fund_id = df.id
LEFT JOIN states s ON t.state_id = s.id
LEFT JOIN zones z ON t.zone_id = z.id
LEFT JOIN users u_init ON t.initiating_actor_id = u_init.id
LEFT JOIN users u_app ON t.approved_by_user_id = u_app.id
WHERE t.id = $1;

-- name: GetFinancialTransactionByReference :one
SELECT * FROM financial_transactions WHERE reference = $1;

-- name: GetFinancialTransactionByIdempotencyKey :one
SELECT * FROM financial_transactions WHERE idempotency_key = $1;

-- name: UpdateFinancialTransactionStatus :one
UPDATE financial_transactions
SET status = $2, approved_by_user_id = $3, approved_at = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateLedgerEntry :one
INSERT INTO financial_ledger_entries (
    id, transaction_id, fund_id, entry_type, amount, currency, description, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListLedgerEntriesByTransaction :many
SELECT e.*, f.name as fund_name
FROM financial_ledger_entries e
JOIN financial_funds f ON e.fund_id = f.id
WHERE e.transaction_id = $1
ORDER BY e.created_at ASC;

-- name: ListFinancialTransactionsFiltered :many
SELECT t.*,
       sf.name as source_fund_name,
       df.name as destination_fund_name,
       s.name as state_name,
       z.name as zone_name,
       u_init.display_name as initiating_actor_name,
       u_app.display_name as approved_by_name
FROM financial_transactions t
LEFT JOIN financial_funds sf ON t.source_fund_id = sf.id
LEFT JOIN financial_funds df ON t.destination_fund_id = df.id
LEFT JOIN states s ON t.state_id = s.id
LEFT JOIN zones z ON t.zone_id = z.id
LEFT JOIN users u_init ON t.initiating_actor_id = u_init.id
LEFT JOIN users u_app ON t.approved_by_user_id = u_app.id
WHERE (sqlc.narg('start_date')::timestamptz IS NULL OR t.occurred_at >= sqlc.narg('start_date')::timestamptz)
  AND (sqlc.narg('end_date')::timestamptz IS NULL OR t.occurred_at <= sqlc.narg('end_date')::timestamptz)
  AND (sqlc.narg('state_id')::varchar IS NULL OR t.state_id = sqlc.narg('state_id')::varchar)
  AND (sqlc.narg('zone_id')::varchar IS NULL OR t.zone_id = sqlc.narg('zone_id')::varchar)
  AND (sqlc.narg('campaign_id')::varchar IS NULL OR t.related_campaign_id = sqlc.narg('campaign_id')::varchar)
  AND (sqlc.narg('project_id')::varchar IS NULL OR t.related_project_id = sqlc.narg('project_id')::varchar)
  AND (sqlc.narg('transaction_type')::varchar IS NULL OR t.transaction_type = sqlc.narg('transaction_type')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR t.status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('fund_id')::varchar IS NULL OR t.source_fund_id = sqlc.narg('fund_id')::varchar OR t.destination_fund_id = sqlc.narg('fund_id')::varchar)
ORDER BY t.occurred_at DESC;

-- name: CalculateFundBalanceBeforeDate :one
SELECT COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount ELSE -amount END), 0)::bigint as balance
FROM financial_ledger_entries
WHERE (sqlc.narg('fund_id')::varchar IS NULL OR fund_id = sqlc.narg('fund_id')::varchar)
  AND created_at < sqlc.arg('before_date')::timestamptz;

-- name: CalculateExternalInflowsAndOutflows :one
SELECT
    COALESCE(SUM(CASE WHEN is_external_income = true AND status = 'posted' THEN amount ELSE 0 END), 0)::bigint as external_inflows,
    COALESCE(SUM(CASE WHEN transaction_type IN ('disbursement', 'expense') AND status = 'posted' THEN amount ELSE 0 END), 0)::bigint as disbursements,
    COALESCE(SUM(CASE WHEN transaction_type = 'provider_fee' AND status = 'posted' THEN amount ELSE 0 END), 0)::bigint as provider_fees,
    COALESCE(SUM(CASE WHEN transaction_type IN ('refund', 'reversal') AND status = 'posted' THEN amount ELSE 0 END), 0)::bigint as refunds_adjustments
FROM financial_transactions
WHERE (sqlc.narg('start_date')::timestamptz IS NULL OR occurred_at >= sqlc.narg('start_date')::timestamptz)
  AND (sqlc.narg('end_date')::timestamptz IS NULL OR occurred_at <= sqlc.narg('end_date')::timestamptz)
  AND (sqlc.narg('state_id')::varchar IS NULL OR state_id = sqlc.narg('state_id')::varchar)
  AND (sqlc.narg('zone_id')::varchar IS NULL OR zone_id = sqlc.narg('zone_id')::varchar)
  AND (sqlc.narg('fund_id')::varchar IS NULL OR source_fund_id = sqlc.narg('fund_id')::varchar OR destination_fund_id = sqlc.narg('fund_id')::varchar);

-- name: CreateFinancialAuthorization :one
INSERT INTO financial_authorizations (
    id, user_id, granted_by_user_id, access_level, status, notes,
    created_at, revoked_at, version
) VALUES (
    $1, $2, $3, $4, 'active', $5,
    CURRENT_TIMESTAMP, NULL, 1
) RETURNING *;

-- name: RevokeFinancialAuthorization :one
UPDATE financial_authorizations
SET status = 'revoked', revoked_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: GetActiveFinancialAuthorization :one
SELECT * FROM financial_authorizations
WHERE user_id = $1 AND status = 'active'
LIMIT 1;

-- name: ListFinancialAuthorizations :many
SELECT fa.*,
       u.display_name as user_name,
       u.email as user_email,
       g.display_name as granted_by_name
FROM financial_authorizations fa
JOIN users u ON fa.user_id = u.id
JOIN users g ON fa.granted_by_user_id = g.id
ORDER BY fa.created_at DESC;

-- name: CreateFinancialReportLog :one
INSERT INTO financial_report_logs (
    id, report_reference, generated_by_user_id, report_type, filters,
    opening_balance, closing_balance, total_inflows, total_outflows, generated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListFinancialReportLogs :many
SELECT rl.*, u.display_name as generated_by_name
FROM financial_report_logs rl
JOIN users u ON rl.generated_by_user_id = u.id
ORDER BY rl.generated_at DESC
LIMIT 50;
