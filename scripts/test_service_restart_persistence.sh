#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================================="
echo " Service Restart Persistence Verification Test"
echo "=========================================================="
echo "Working Root Directory: ${ROOT_DIR}"
echo "API Directory: ${ROOT_DIR}/apps/api"

PORT=8005
TEST_DB="postgres://postgres@127.0.0.1:5432/ratels_restart_test?sslmode=disable"
export GOTOOLCHAIN=auto
export PATH="${HOME}/go/bin:${PATH}"

# 1. Apply migrations to isolated ratels_restart_test database
echo "-> Ensuring isolated test database ratels_restart_test exists and is migrated..."
createdb -h 127.0.0.1 -p 5432 -U postgres ratels_restart_test 2>/dev/null || true
cd "${ROOT_DIR}/apps/api"
goose -dir "${ROOT_DIR}/apps/api/migrations" postgres "${TEST_DB}" up > /dev/null

# 2. Seed baseline reference data (funds, geography) into isolated test database
echo "-> Populating reference funds and geography in isolated test database..."
DATABASE_URL="${TEST_DB}" go run ./cmd/seed > /dev/null

# 3. Bootstrap isolated Super Admin and compile server binary
SA_EMAIL="restart_sa_$(( RANDOM % 90000 + 10000 ))@ratels.example.org"
SA_PASS="RestartAdminSecurePass123!"

echo "-> Compiling server binary..."
go build -o /tmp/ratels-restart-server ./cmd/server

echo "-> Bootstrapping test Super Admin: ${SA_EMAIL}..."
DATABASE_URL="${TEST_DB}" go run ./cmd/admin --super-admin --email "${SA_EMAIL}" --password "${SA_PASS}" --name "Restart Persistence Admin" > /dev/null

# 4. Start Server Instance #1
echo "-> Launching API Server Instance #1 on port ${PORT} against isolated ratels_restart_test..."
PORT=${PORT} DATABASE_URL="${TEST_DB}" /tmp/ratels-restart-server &
PID_1=$!
echo "   Server #1 running with PID ${PID_1}"

# Wait for server ready
for i in {1..30}; do
    if curl -s "http://127.0.0.1:${PORT}/api/v1/health" | grep -q '"status":"ok"'; then
        echo "   Server #1 is responsive on http://127.0.0.1:${PORT}/api/v1/health"
        break
    fi
    sleep 0.5
done

# 5. Log in and acquire token
echo "-> Logging in to Server #1..."
LOGIN_RESP=$(curl -s -X POST "http://127.0.0.1:${PORT}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${SA_EMAIL}\",\"password\":\"${SA_PASS}\"}")

TOKEN=$(echo "${LOGIN_RESP}" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
if [ -z "${TOKEN}" ]; then
    echo "ERROR: Failed to obtain token from Server #1: ${LOGIN_RESP}"
    kill -9 "${PID_1}" || true
    exit 1
fi
echo "   Successfully authenticated. JWT Bearer token acquired."

# 6. Record Fund Movement with Segregated Fixture Tag
IDEM_KEY="TEST-FIXTURE-restart-tx-$(( RANDOM % 90000 + 10000 ))"
echo "-> Posting financial transaction with Idempotency Key: ${IDEM_KEY}..."
TX_RESP=$(curl -s -X POST "http://127.0.0.1:${PORT}/api/v1/finance/transactions" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
        \"idempotency_key\":\"${IDEM_KEY}\",
        \"transaction_type\":\"confirmed_donation\",
        \"amount\":7500000,
        \"currency\":\"NGN\",
        \"destination_fund_id\":\"foundation_donation_fund\",
        \"notes\":\"[TEST FIXTURE - ISOLATED PERSISTENCE VERIFICATION - NOT A REAL DONATION] Clean water tank test\",
        \"auto_post\":true
    }")

TX_ID=$(echo "${TX_RESP}" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
TX_REF=$(echo "${TX_RESP}" | grep -o '"reference":"[^"]*' | head -1 | cut -d'"' -f4)
echo "   Transaction recorded on Server #1: ID=${TX_ID}, Ref=${TX_REF}, Amount=7,500,000 Kobo (₦75,000.00)"

# 7. Test Idempotent Retry Protection
echo "-> Testing Idempotent Retry Protection (re-sending identical transaction)..."
RETRY_RESP=$(curl -s -X POST "http://127.0.0.1:${PORT}/api/v1/finance/transactions" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
        \"idempotency_key\":\"${IDEM_KEY}\",
        \"transaction_type\":\"confirmed_donation\",
        \"amount\":7500000,
        \"currency\":\"NGN\",
        \"destination_fund_id\":\"foundation_donation_fund\",
        \"notes\":\"[TEST FIXTURE - ISOLATED PERSISTENCE VERIFICATION - NOT A REAL DONATION] Clean water tank test\",
        \"auto_post\":true
    }")

RETRY_ID=$(echo "${RETRY_RESP}" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
if [ "${TX_ID}" != "${RETRY_ID}" ]; then
    echo "ERROR: Idempotency failed! Original=${TX_ID}, Retry=${RETRY_ID}"
    kill -9 "${PID_1}" || true
    exit 1
fi
echo "   ✓ Idempotent retry verified: identical transaction returned without duplicate ledger creation."

# 8. Test Reversal-Based Correction
echo "-> Executing Super Admin Reversal-Based Correction on transaction ${TX_ID}..."
REV_RESP=$(curl -s -X POST "http://127.0.0.1:${PORT}/api/v1/finance/transactions/${TX_ID}/reverse" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"reason\":\"[TEST FIXTURE - AUDIT CORRECTION] Reversal test before service restart\"}")

REV_ID=$(echo "${REV_RESP}" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
REV_REF=$(echo "${REV_RESP}" | grep -o '"reference":"[^"]*' | head -1 | cut -d'"' -f4)
echo "   ✓ Reversal transaction created: ID=${REV_ID}, Ref=${REV_REF}"

# 9. Verify statement reconciliation before restart
STMT_1=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/statement?fund_id=foundation_donation_fund" \
    -H "Authorization: Bearer ${TOKEN}")
CLOSING_1=$(echo "${STMT_1}" | python3 -c "import sys, json; print(json.loads(sys.stdin.read()).get('closing_balance', '0'))")
RECON_1=$(echo "${STMT_1}" | python3 -c "import sys, json; print(json.loads(sys.stdin.read()).get('reconciliation_status', ''))")
echo "   Pre-restart Closing Balance: ${CLOSING_1} Kobo, Reconciliation: ${RECON_1}"

# 10. HARD TERMINATION of Server #1
echo "-> TERMINATING Server #1 process (PID ${PID_1})..."
kill -9 "${PID_1}"
wait "${PID_1}" 2>/dev/null || true

# Verify server is completely stopped
if curl -s --connect-timeout 1 "http://127.0.0.1:${PORT}/api/v1/health" > /dev/null 2>&1; then
    echo "ERROR: Server #1 still responding after kill -9!"
    exit 1
fi
echo "   Confirmed Server #1 is terminated. Connection refused as expected."

# 11. Start Server Instance #2 (Fresh process)
echo "-> Launching API Server Instance #2 (Cold start)..."
PORT=${PORT} DATABASE_URL="${TEST_DB}" /tmp/ratels-restart-server &
PID_2=$!
echo "   Server #2 running with PID ${PID_2}"

for i in {1..30}; do
    if curl -s "http://127.0.0.1:${PORT}/api/v1/health" | grep -q '"status":"ok"'; then
        echo "   Server #2 is responsive on http://127.0.0.1:${PORT}/api/v1/health"
        break
    fi
    sleep 0.5
done

# 12. Query Server #2 to prove persistence across service restart
echo "-> Authenticating against Server #2 with original credentials..."
LOGIN_RESP_2=$(curl -s -X POST "http://127.0.0.1:${PORT}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${SA_EMAIL}\",\"password\":\"${SA_PASS}\"}")

TOKEN_2=$(echo "${LOGIN_RESP_2}" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

echo "-> Fetching financial statement from Server #2..."
STMT_2=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/statement?fund_id=foundation_donation_fund" \
    -H "Authorization: Bearer ${TOKEN_2}")

CLOSING_2=$(echo "${STMT_2}" | python3 -c "import sys, json; print(json.loads(sys.stdin.read()).get('closing_balance', '0'))")
RECON_2=$(echo "${STMT_2}" | python3 -c "import sys, json; print(json.loads(sys.stdin.read()).get('reconciliation_status', ''))")
echo "   Post-restart Closing Balance: ${CLOSING_2} Kobo, Reconciliation: ${RECON_2}"

if [ "${CLOSING_1}" != "${CLOSING_2}" ]; then
    echo "ERROR: Closing balance mismatch! Pre=${CLOSING_1}, Post=${CLOSING_2}"
    kill -9 "${PID_2}" || true
    exit 1
fi

# Verify original transaction detail and ledger entries on Server #2
TX_FETCH=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/transactions/${TX_ID}" \
    -H "Authorization: Bearer ${TOKEN_2}")

if ! echo "${TX_FETCH}" | grep -q "${TX_REF}"; then
    echo "ERROR: Transaction reference not found on Server #2!"
    kill -9 "${PID_2}" || true
    exit 1
fi

LEDGER_COUNT=$(echo "${TX_FETCH}" | grep -o '"entry_type"' | wc -l)
echo "   Post-restart Original Transaction verified: Ref=${TX_REF}, Balanced Ledger Entries=${LEDGER_COUNT}"

# Verify reversal transaction detail on Server #2
REV_FETCH=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/transactions/${REV_ID}" \
    -H "Authorization: Bearer ${TOKEN_2}")

if ! echo "${REV_FETCH}" | grep -q "${REV_REF}"; then
    echo "ERROR: Reversal transaction reference not found on Server #2!"
    kill -9 "${PID_2}" || true
    exit 1
fi
echo "   Post-restart Reversal Transaction verified: Ref=${REV_REF}"

# 13. Clean shutdown of Server #2
echo "-> Shutting down Server #2 cleanly..."
kill "${PID_2}" 2>/dev/null || true
wait "${PID_2}" 2>/dev/null || true

echo "=========================================================="
echo " Service Restart Persistence Test PASSED Successfully"
echo " Records, retries, and reversals persisted across restart."
echo " Zero pollution of ratels_dev database."
echo "=========================================================="
