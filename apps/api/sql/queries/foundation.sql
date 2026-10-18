-- name: GetFoundationEntity :one
SELECT * FROM foundation_entities
LIMIT 1;

-- name: CreateFoundationEntity :one
INSERT INTO foundation_entities (
    id, provisional_name, registered_legal_name, registration_number,
    is_verified, mission_statement, disclaimer, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: UpdateFoundationEntity :one
UPDATE foundation_entities
SET provisional_name = $2, registered_legal_name = $3, registration_number = $4,
    is_verified = $5, mission_statement = $6, disclaimer = $7, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: ListVerifiedBankAccountVersions :many
SELECT * FROM verified_bank_account_versions
ORDER BY created_at DESC;

-- name: CreateBankAccountVersion :one
INSERT INTO verified_bank_account_versions (
    id, bank_name, account_name, account_number, currency, status,
    submitted_by_user_id, submitted_by_officer,
    verified_by_officer_1, verified_by_officer_2, effective_date, verification_notes,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetBankAccountVersionByID :one
SELECT * FROM verified_bank_account_versions
WHERE id = $1;

-- name: UpdateBankAccountVerification :one
UPDATE verified_bank_account_versions
SET status = $2, verified_by_officer_1 = $3, verified_by_officer_2 = $4,
    effective_date = $5, verification_notes = $6, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: ListFoundationCampaigns :many
SELECT * FROM foundation_campaigns
ORDER BY created_at DESC;

-- name: CreateFoundationCampaign :one
INSERT INTO foundation_campaigns (
    id, title, purpose, target_amount_kobo, status, is_live_donations_enabled,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;
