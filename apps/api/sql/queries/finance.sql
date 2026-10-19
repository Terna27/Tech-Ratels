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

-- name: CreateFinancialTransactionWithIncome :one
INSERT INTO financial_transactions (
    id, reference, idempotency_key, transaction_type, status, amount, currency,
    source_fund_id, destination_fund_id, related_campaign_id, related_project_id,
    state_id, zone_id, lga_id, provider_reference, supporting_evidence_url, notes,
    initiating_actor_id, initiating_actor_type, approved_by_user_id, approved_at,
    is_external_income, occurred_at, recorded_at, reversal_of_transaction_id,
    is_reversal, reversal_reason, income_category, payment_method, funding_source,
    receiving_account_id, verification_status, verified_by_user_id, verified_at,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11,
    $12, $13, $14, $15, $16, $17,
    $18, $19, $20, $21,
    $22, $23, CURRENT_TIMESTAMP, $24,
    $25, $26, $27, $28, $29,
    $30, $31, $32, $33,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: VerifyFinancialIncomeTransaction :one
UPDATE financial_transactions
SET verification_status = $2,
    status = $3,
    verified_by_user_id = $4,
    verified_at = $5,
    approved_by_user_id = $4,
    approved_at = $5,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateBankAccount :one
INSERT INTO financial_bank_accounts (
    id, bank_name, account_name, account_number, currency, org_type, status, is_primary, notes,
    created_by_user_id, activated_by_user_id, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetBankAccountByID :one
SELECT * FROM financial_bank_accounts WHERE id = $1;

-- name: ListBankAccounts :many
SELECT * FROM financial_bank_accounts
WHERE (sqlc.narg('org_type')::varchar IS NULL OR org_type = sqlc.narg('org_type')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
ORDER BY is_primary DESC, created_at ASC;

-- name: ActivateBankAccount :one
UPDATE financial_bank_accounts
SET status = 'active', activated_by_user_id = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateBudget :one
INSERT INTO financial_budgets (
    id, reference, title, scope_type, scope_id, zone_id, state_id, lga_id,
    period_type, period_label, period_start, period_end, proposed_amount_kobo,
    approved_amount_kobo, allocated_amount_kobo, committed_amount_kobo, spent_amount_kobo,
    currency, status, created_by_user_id, approved_by_user_id, approved_at, notes,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19, $20, $21, $22, $23,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetBudgetByID :one
SELECT b.*,
       z.name as zone_name,
       s.name as state_name,
       l.name as lga_name,
       u_creator.display_name as creator_name,
       u_approver.display_name as approver_name
FROM financial_budgets b
LEFT JOIN zones z ON b.zone_id = z.id
LEFT JOIN states s ON b.state_id = s.id
LEFT JOIN lgas l ON b.lga_id = l.id
LEFT JOIN users u_creator ON b.created_by_user_id = u_creator.id
LEFT JOIN users u_approver ON b.approved_by_user_id = u_approver.id
WHERE b.id = $1;

-- name: GetBudgetByReference :one
SELECT * FROM financial_budgets WHERE reference = $1;

-- name: ListBudgetsFiltered :many
SELECT b.*,
       z.name as zone_name,
       s.name as state_name,
       l.name as lga_name,
       u_creator.display_name as creator_name,
       u_approver.display_name as approver_name
FROM financial_budgets b
LEFT JOIN zones z ON b.zone_id = z.id
LEFT JOIN states s ON b.state_id = s.id
LEFT JOIN lgas l ON b.lga_id = l.id
LEFT JOIN users u_creator ON b.created_by_user_id = u_creator.id
LEFT JOIN users u_approver ON b.approved_by_user_id = u_approver.id
WHERE (sqlc.narg('scope_type')::varchar IS NULL OR b.scope_type = sqlc.narg('scope_type')::varchar)
  AND (sqlc.narg('scope_id')::varchar IS NULL OR b.scope_id = sqlc.narg('scope_id')::varchar)
  AND (sqlc.narg('period_label')::varchar IS NULL OR b.period_label = sqlc.narg('period_label')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR b.status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('zone_id')::varchar IS NULL OR b.zone_id = sqlc.narg('zone_id')::varchar)
  AND (sqlc.narg('state_id')::varchar IS NULL OR b.state_id = sqlc.narg('state_id')::varchar)
ORDER BY b.created_at DESC;

-- name: UpdateBudgetStatus :one
UPDATE financial_budgets
SET status = sqlc.arg('status')::varchar,
    approved_amount_kobo = CASE WHEN sqlc.arg('status')::varchar = 'approved' AND approved_amount_kobo = 0 THEN proposed_amount_kobo ELSE approved_amount_kobo END,
    approved_by_user_id = $2,
    approved_at = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateBudgetApprovedAmount :one
UPDATE financial_budgets
SET approved_amount_kobo = $2,
    status = 'approved',
    approved_by_user_id = $3,
    approved_at = $4,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;


-- name: UpdateBudgetAmounts :one
UPDATE financial_budgets
SET allocated_amount_kobo = allocated_amount_kobo + $2,
    committed_amount_kobo = committed_amount_kobo + $3,
    spent_amount_kobo = spent_amount_kobo + $4,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateBudgetRevision :one
INSERT INTO financial_budget_revisions (
    id, budget_id, revision_number, previous_amount_kobo, revised_amount_kobo,
    reason, requested_by_user_id, approved_by_user_id, approved_at, status, created_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListBudgetRevisions :many
SELECT r.*,
       u_req.display_name as requester_name,
       u_app.display_name as approver_name
FROM financial_budget_revisions r
LEFT JOIN users u_req ON r.requested_by_user_id = u_req.id
LEFT JOIN users u_app ON r.approved_by_user_id = u_app.id
WHERE r.budget_id = $1
ORDER BY r.revision_number DESC;

-- name: UpdateBudgetRevisionStatus :one
UPDATE financial_budget_revisions
SET status = $2, approved_by_user_id = $3, approved_at = $4
WHERE id = $1
RETURNING *;

-- name: CreateExpense :one
INSERT INTO financial_expenses (
    id, reference, title, category, description, amount_kobo, currency,
    beneficiary_name, beneficiary_bank_name, beneficiary_account_number,
    zone_id, state_id, lga_id, related_project_id, related_campaign_id,
    supporting_document_url, receipt_url, requested_by_user_id,
    budget_id, fund_id, status, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18,
    $19, $20, $21, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetExpenseByID :one
SELECT e.*,
       z.name as zone_name,
       s.name as state_name,
       l.name as lga_name,
       cp.title as related_project_title,
       cc.title as related_campaign_title,
       u_req.display_name as requested_by_name,
       u_rev.display_name as reviewer_name,
       u_app.display_name as approved_by_name,
       u_auth.display_name as authorized_by_name,
       u_paid.display_name as paid_by_name,
       f.name as fund_name
FROM financial_expenses e
LEFT JOIN zones z ON e.zone_id = z.id
LEFT JOIN states s ON e.state_id = s.id
LEFT JOIN lgas l ON e.lga_id = l.id
LEFT JOIN community_projects cp ON e.related_project_id = cp.id
LEFT JOIN cleanup_campaigns cc ON e.related_campaign_id = cc.id
LEFT JOIN users u_req ON e.requested_by_user_id = u_req.id
LEFT JOIN users u_rev ON e.reviewer_id = u_rev.id
LEFT JOIN users u_app ON e.approved_by_user_id = u_app.id
LEFT JOIN users u_auth ON e.authorized_by_user_id = u_auth.id
LEFT JOIN users u_paid ON e.paid_by_user_id = u_paid.id
LEFT JOIN financial_funds f ON e.fund_id = f.id
WHERE e.id = $1;

-- name: GetExpenseByReference :one
SELECT * FROM financial_expenses WHERE reference = $1;

-- name: ListExpensesFiltered :many
SELECT e.*,
       z.name as zone_name,
       s.name as state_name,
       l.name as lga_name,
       u_req.display_name as requested_by_name,
       u_app.display_name as approved_by_name
FROM financial_expenses e
LEFT JOIN zones z ON e.zone_id = z.id
LEFT JOIN states s ON e.state_id = s.id
LEFT JOIN lgas l ON e.lga_id = l.id
LEFT JOIN users u_req ON e.requested_by_user_id = u_req.id
LEFT JOIN users u_app ON e.approved_by_user_id = u_app.id
WHERE (sqlc.narg('category')::varchar IS NULL OR e.category = sqlc.narg('category')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR e.status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('zone_id')::varchar IS NULL OR e.zone_id = sqlc.narg('zone_id')::varchar)
  AND (sqlc.narg('state_id')::varchar IS NULL OR e.state_id = sqlc.narg('state_id')::varchar)
  AND (sqlc.narg('lga_id')::varchar IS NULL OR e.lga_id = sqlc.narg('lga_id')::varchar)
  AND (sqlc.narg('related_project_id')::varchar IS NULL OR e.related_project_id = sqlc.narg('related_project_id')::varchar)
  AND (sqlc.narg('budget_id')::varchar IS NULL OR e.budget_id = sqlc.narg('budget_id')::varchar)
  AND (sqlc.narg('requested_by_user_id')::varchar IS NULL OR e.requested_by_user_id = sqlc.narg('requested_by_user_id')::varchar)
ORDER BY e.created_at DESC;

-- name: UpdateExpenseReview :one
UPDATE financial_expenses
SET status = $2, reviewer_id = $3, reviewed_at = $4, rejection_reason = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateExpenseApproval :one
UPDATE financial_expenses
SET status = $2, approved_by_user_id = $3, approved_at = $4, rejection_reason = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateExpenseAuthorization :one
UPDATE financial_expenses
SET status = 'authorized_for_payment', authorized_by_user_id = $2, authorized_at = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateExpensePayment :one
UPDATE financial_expenses
SET status = 'paid', paid_by_user_id = $2, paid_at = $3, payment_reference = $4, transaction_id = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateAllocation :one
INSERT INTO financial_allocations (
    id, reference, source_type, source_id, destination_type, destination_id,
    source_fund_id, destination_fund_id, amount_kobo, currency, purpose,
    approval_reference, transaction_id, allocated_by_user_id, status, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11,
    $12, $13, $14, $15, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetAllocationByID :one
SELECT a.*,
       sf.name as source_fund_name,
       df.name as destination_fund_name,
       u.display_name as allocated_by_name
FROM financial_allocations a
LEFT JOIN financial_funds sf ON a.source_fund_id = sf.id
LEFT JOIN financial_funds df ON a.destination_fund_id = df.id
LEFT JOIN users u ON a.allocated_by_user_id = u.id
WHERE a.id = $1;

-- name: ListAllocationsFiltered :many
SELECT a.*,
       sf.name as source_fund_name,
       df.name as destination_fund_name,
       u.display_name as allocated_by_name
FROM financial_allocations a
LEFT JOIN financial_funds sf ON a.source_fund_id = sf.id
LEFT JOIN financial_funds df ON a.destination_fund_id = df.id
LEFT JOIN users u ON a.allocated_by_user_id = u.id
WHERE (sqlc.narg('source_type')::varchar IS NULL OR a.source_type = sqlc.narg('source_type')::varchar)
  AND (sqlc.narg('destination_type')::varchar IS NULL OR a.destination_type = sqlc.narg('destination_type')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR a.status = sqlc.narg('status')::varchar)
ORDER BY a.created_at DESC;

-- name: CreateDonation :one
INSERT INTO financial_donations (
    id, reference, entity_type, campaign_id, donor_name, donor_email, donor_phone,
    is_anonymous, amount_kobo, currency, payment_method, payment_reference, provider_channel,
    status, zone_id, state_id, destination_fund_id, transaction_id, verified_at,
    receipt_number, notes, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19,
    $20, $21, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetDonationByID :one
SELECT d.*,
       fc.title as campaign_title,
       z.name as zone_name,
       s.name as state_name,
       df.name as destination_fund_name
FROM financial_donations d
LEFT JOIN foundation_campaigns fc ON d.campaign_id = fc.id
LEFT JOIN zones z ON d.zone_id = z.id
LEFT JOIN states s ON d.state_id = s.id
LEFT JOIN financial_funds df ON d.destination_fund_id = df.id
WHERE d.id = $1;

-- name: GetDonationByReference :one
SELECT * FROM financial_donations WHERE reference = $1;

-- name: GetDonationByPaymentReference :one
SELECT * FROM financial_donations WHERE payment_reference = $1;

-- name: UpdateDonationStatus :one
UPDATE financial_donations
SET status = $2, verified_at = $3, transaction_id = $4, receipt_number = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: ListDonationsFiltered :many
SELECT d.*,
       fc.title as campaign_title,
       z.name as zone_name,
       s.name as state_name
FROM financial_donations d
LEFT JOIN foundation_campaigns fc ON d.campaign_id = fc.id
LEFT JOIN zones z ON d.zone_id = z.id
LEFT JOIN states s ON d.state_id = s.id
WHERE (sqlc.narg('entity_type')::varchar IS NULL OR d.entity_type = sqlc.narg('entity_type')::varchar)
  AND (sqlc.narg('campaign_id')::varchar IS NULL OR d.campaign_id = sqlc.narg('campaign_id')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR d.status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('zone_id')::varchar IS NULL OR d.zone_id = sqlc.narg('zone_id')::varchar)
  AND (sqlc.narg('state_id')::varchar IS NULL OR d.state_id = sqlc.narg('state_id')::varchar)
ORDER BY d.created_at DESC;

-- name: GetDonationCampaignTotals :one
SELECT
    COUNT(*)::bigint as total_count,
    COALESCE(SUM(CASE WHEN status = 'verified' THEN amount_kobo ELSE 0 END), 0)::bigint as verified_total_kobo
FROM financial_donations
WHERE campaign_id = $1;

-- name: CreateReconciliationPeriod :one
INSERT INTO financial_reconciliation_periods (
    id, bank_account_id, period_start, period_end, opening_balance_kobo,
    closing_balance_kobo, statement_closing_balance_kobo, status, notes,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetReconciliationPeriodByID :one
SELECT rp.*,
       ba.bank_name,
       ba.account_name,
       ba.account_number,
       ba.org_type,
       u_rec.display_name as reconciled_by_name,
       u_cert.display_name as certified_by_name
FROM financial_reconciliation_periods rp
JOIN financial_bank_accounts ba ON rp.bank_account_id = ba.id
LEFT JOIN users u_rec ON rp.reconciled_by_user_id = u_rec.id
LEFT JOIN users u_cert ON rp.certified_by_user_id = u_cert.id
WHERE rp.id = $1;

-- name: ListReconciliationPeriodsByAccount :many
SELECT rp.*,
       ba.bank_name,
       ba.account_name,
       u_rec.display_name as reconciled_by_name,
       u_cert.display_name as certified_by_name
FROM financial_reconciliation_periods rp
JOIN financial_bank_accounts ba ON rp.bank_account_id = ba.id
LEFT JOIN users u_rec ON rp.reconciled_by_user_id = u_rec.id
LEFT JOIN users u_cert ON rp.certified_by_user_id = u_cert.id
WHERE (sqlc.narg('bank_account_id')::varchar IS NULL OR rp.bank_account_id = sqlc.narg('bank_account_id')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR rp.status = sqlc.narg('status')::varchar)
ORDER BY rp.period_start DESC;

-- name: UpdateReconciliationPeriodStatus :one
UPDATE financial_reconciliation_periods
SET status = $2,
    reconciled_by_user_id = COALESCE($3, reconciled_by_user_id),
    reconciled_at = COALESCE($4, reconciled_at),
    certified_by_user_id = COALESCE($5, certified_by_user_id),
    certified_at = COALESCE($6, certified_at),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateStatementItem :one
INSERT INTO financial_statement_items (
    id, reconciliation_period_id, statement_reference, transaction_date,
    value_date, description, amount_kobo, entry_type, matched_transaction_id,
    status, match_notes, created_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9,
    $10, $11, CURRENT_TIMESTAMP
) RETURNING *;

-- name: ListStatementItemsByPeriod :many
SELECT si.*,
       ft.reference as matched_tx_reference,
       ft.amount as matched_tx_amount
FROM financial_statement_items si
LEFT JOIN financial_transactions ft ON si.matched_transaction_id = ft.id
WHERE si.reconciliation_period_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR si.status = sqlc.narg('status')::varchar)
ORDER BY si.transaction_date ASC, si.created_at ASC;

-- name: UpdateStatementItemMatch :one
UPDATE financial_statement_items
SET status = $2, matched_transaction_id = $3, match_notes = $4
WHERE id = $1
RETURNING *;

-- name: GetFinancialDashboardSummary :one
SELECT
    COALESCE(SUM(CASE WHEN t.is_external_income = true AND t.status = 'posted' THEN t.amount ELSE 0 END), 0)::bigint as total_verified_income,
    COALESCE(SUM(CASE WHEN t.transaction_type IN ('disbursement', 'expense') AND t.status = 'posted' THEN t.amount ELSE 0 END), 0)::bigint as total_verified_expenditure,
    COALESCE(SUM(CASE WHEN e.status IN ('submitted', 'under_review') THEN e.amount_kobo ELSE 0 END), 0)::bigint as outstanding_commitments,
    COALESCE((SELECT COUNT(*) FROM financial_expenses WHERE status IN ('submitted', 'under_review')), 0)::bigint as pending_expenses_count,
    COALESCE((SELECT COUNT(*) FROM financial_transactions WHERE status = 'pending'), 0)::bigint as pending_transactions_count,
    COALESCE((SELECT COUNT(*) FROM financial_reconciliation_periods WHERE status IN ('draft', 'in_progress')), 0)::bigint as pending_reconciliations_count,
    COALESCE((SELECT SUM(approved_amount_kobo) FROM financial_budgets WHERE status = 'approved'), 0)::bigint as active_budgets_total,
    COALESCE((SELECT SUM(allocated_funding) FROM community_projects), 0)::bigint as project_allocations_total,
    COALESCE((SELECT SUM(amount_kobo) FROM financial_donations WHERE status = 'verified'), 0)::bigint as total_donations_received
FROM financial_transactions t
FULL OUTER JOIN financial_expenses e ON FALSE;
