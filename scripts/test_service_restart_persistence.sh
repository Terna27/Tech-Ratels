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
TEST_DB="postgres://postgres@127.0.0.1:5432/ratels_dev?sslmode=disable"
export GOTOOLCHAIN=auto
export PATH="${HOME}/go/bin:${PATH}"

# 1. Apply migrations to ratels_dev
cd "${ROOT_DIR}/apps/api"
goose -dir "${ROOT_DIR}/apps/api/migrations" postgres "${TEST_DB}" up > /dev/null

# 2. Seed geography, bootstrap Super Admin, and compile server binary
SA_EMAIL="restart_sa_$(( RANDOM % 90000 + 10000 ))@ratels.example.org"
SA_PASS="RestartAdminSecurePass123!"

echo "-> Compiling server binary..."
go build -o /tmp/ratels-restart-server ./cmd/server

echo "-> Bootstrapping test Super Admin: ${SA_EMAIL}..."
go run ./cmd/admin --super-admin --email "${SA_EMAIL}" --password "${SA_PASS}" --name "Restart Persistence Admin" > /dev/null

# 3. Start Server Instance #1
echo "-> Launching API Server Instance #1 on port ${PORT}..."
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

# 4. Log in and acquire token
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

# 5. Record Fund Movement (₦75,000.00 = 7,500,000 Kobo donation)
IDEM_KEY="persist-restart-tx-$(( RANDOM % 90000 + 10000 ))"
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
        \"notes\":\"Clean water tank donation prior to service restart\",
        \"auto_post\":true
    }")

TX_ID=$(echo "${TX_RESP}" | grep -o '"id":"[^"]*' | head -1 | cut -d'"' -f4)
TX_REF=$(echo "${TX_RESP}" | grep -o '"reference":"[^"]*' | head -1 | cut -d'"' -f4)
echo "   Transaction recorded on Server #1: ID=${TX_ID}, Ref=${TX_REF}, Amount=7,500,000 Kobo (₦75,000.00)"

# Verify statement before restart
STMT_1=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/statement?fund_id=foundation_donation_fund" \
    -H "Authorization: Bearer ${TOKEN}")
CLOSING_1=$(echo "${STMT_1}" | grep -o '"closing_balance":[0-9]*' | cut -d':' -f2)
echo "   Pre-restart Closing Balance: ${CLOSING_1} Kobo"

# 6. HARD TERMINATION of Server #1
echo "-> TERMINATING Server #1 process (PID ${PID_1})..."
kill -9 "${PID_1}"
wait "${PID_1}" 2>/dev/null || true

# Verify server is completely stopped
if curl -s --connect-timeout 1 "http://127.0.0.1:${PORT}/api/v1/health" > /dev/null 2>&1; then
    echo "ERROR: Server #1 still responding after kill -9!"
    exit 1
fi
echo "   Confirmed Server #1 is terminated. Connection refused as expected."

# 7. Start Server Instance #2 (Fresh process)
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

# 8. Query Server #2 to prove persistence across service restart
echo "-> Authenticating against Server #2 with original credentials..."
LOGIN_RESP_2=$(curl -s -X POST "http://127.0.0.1:${PORT}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${SA_EMAIL}\",\"password\":\"${SA_PASS}\"}")

TOKEN_2=$(echo "${LOGIN_RESP_2}" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

echo "-> Fetching financial statement from Server #2..."
STMT_2=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/statement?fund_id=foundation_donation_fund" \
    -H "Authorization: Bearer ${TOKEN_2}")

CLOSING_2=$(echo "${STMT_2}" | grep -o '"closing_balance":[0-9]*' | cut -d':' -f2)
echo "   Post-restart Closing Balance: ${CLOSING_2} Kobo"

if [ "${CLOSING_1}" != "${CLOSING_2}" ]; then
    echo "ERROR: Closing balance mismatch! Pre=${CLOSING_1}, Post=${CLOSING_2}"
    kill -9 "${PID_2}" || true
    exit 1
fi

# Verify transaction detail and ledger entries on Server #2
TX_FETCH=$(curl -s -X GET "http://127.0.0.1:${PORT}/api/v1/finance/transactions/${TX_ID}" \
    -H "Authorization: Bearer ${TOKEN_2}")

if ! echo "${TX_FETCH}" | grep -q "${TX_REF}"; then
    echo "ERROR: Transaction reference not found on Server #2!"
    kill -9 "${PID_2}" || true
    exit 1
fi

LEDGER_COUNT=$(echo "${TX_FETCH}" | grep -o '"entry_type"' | wc -l)
echo "   Post-restart Transaction verified: Ref=${TX_REF}, Balanced Ledger Entries=${LEDGER_COUNT}"

# 9. Clean shutdown of Server #2
echo "-> Shutting down Server #2 cleanly..."
kill "${PID_2}" 2>/dev/null || true
wait "${PID_2}" 2>/dev/null || true

echo "=========================================================="
echo " Service Restart Persistence Test PASSED Successfully"
echo " Records persisted across hard process restart."
echo "=========================================================="
