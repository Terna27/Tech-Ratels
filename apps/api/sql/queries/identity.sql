-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1;

-- name: GetUserByMembershipReference :one
SELECT * FROM users WHERE membership_reference = $1;

-- name: CreateUser :one
INSERT INTO users (
    id, email, phone, password_hash, display_name, membership_reference,
    status, primary_role, is_verified, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: UpdateUserStatus :one
UPDATE users
SET status = $2, is_verified = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateMemberProfile :one
INSERT INTO member_profiles (
    id, user_id, home_state_id, home_lga_id, address, bio, avatar_url,
    is_public_badges, created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetMemberProfileByUserID :one
SELECT * FROM member_profiles WHERE user_id = $1;

-- name: UpdateMemberProfile :one
UPDATE member_profiles
SET home_state_id = $2, home_lga_id = $3, address = $4, bio = $5, avatar_url = $6, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING *;

-- name: CreateContactVerification :one
INSERT INTO contact_verifications (
    id, user_id, identifier, code, expires_at, is_used, attempt_count,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetActiveVerification :one
SELECT * FROM contact_verifications
WHERE user_id = $1 AND identifier = $2 AND is_used = false AND expires_at > CURRENT_TIMESTAMP
ORDER BY created_at DESC
LIMIT 1;

-- name: MarkVerificationUsed :one
UPDATE contact_verifications
SET is_used = true, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: IncrementVerificationAttempt :one
UPDATE contact_verifications
SET attempt_count = attempt_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: CreateUserSession :one
INSERT INTO user_sessions (
    id, user_id, token_jti, is_revoked, expires_at, ip_address, user_agent,
    created_at, updated_at, version
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1
) RETURNING *;

-- name: GetUserSession :one
SELECT * FROM user_sessions
WHERE token_jti = $1 AND is_revoked = false AND expires_at > CURRENT_TIMESTAMP;

-- name: RevokeUserSession :exec
UPDATE user_sessions
SET is_revoked = true, updated_at = CURRENT_TIMESTAMP
WHERE token_jti = $1;

-- name: RevokeAllUserSessions :exec
UPDATE user_sessions
SET is_revoked = true, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountActiveUsers :one
SELECT COUNT(*) FROM users WHERE status = 'active';

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: CountPendingVerificationUsers :one
SELECT COUNT(*) FROM users WHERE status = 'pending_verification' OR is_verified = false;

-- name: CreateEmailVerificationToken :one
INSERT INTO email_verification_tokens (
    id, user_id, email, token_hash, expires_at, is_used, created_at
) VALUES (
    $1, $2, $3, $4, $5, false, CURRENT_TIMESTAMP
) RETURNING *;

-- name: GetEmailVerificationTokenByHash :one
SELECT * FROM email_verification_tokens
WHERE token_hash = $1
LIMIT 1;

-- name: MarkEmailVerificationTokenUsed :one
UPDATE email_verification_tokens
SET is_used = true, used_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: InvalidatePendingEmailVerificationTokens :exec
UPDATE email_verification_tokens
SET is_used = true, used_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND is_used = false;

-- name: GetVerificationRateLimit :one
SELECT * FROM email_verification_rate_limits
WHERE identifier = $1 AND action = $2
LIMIT 1;

-- name: UpsertVerificationRateLimit :one
INSERT INTO email_verification_rate_limits (
    id, identifier, action, request_count, first_requested_at, last_requested_at
) VALUES (
    $1, $2, $3, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT (identifier, action) DO UPDATE
SET request_count = email_verification_rate_limits.request_count + 1,
    last_requested_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: ResetVerificationRateLimit :one
UPDATE email_verification_rate_limits
SET request_count = 1, first_requested_at = CURRENT_TIMESTAMP, last_requested_at = CURRENT_TIMESTAMP
WHERE identifier = $1 AND action = $2
RETURNING *;
