#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================================="
echo " Launching Community Projects Browser Acceptance Suite"
echo "=========================================================="

PORT=8000
WEB_PORT=3000
TEST_DB="postgres://postgres@127.0.0.1:5432/ratels_browser_test?sslmode=disable"
export GOTOOLCHAIN=auto
export PATH="${HOME}/go/bin:${PATH}"

# 1. Ensure PostgreSQL is active
if ! pg_isready -h 127.0.0.1 -p 5432 -q; then
    echo "-> Starting local PostgreSQL instance..."
    if [ -d "${HOME}/.local/share/ratels_pgdata" ]; then
        pg_ctl -D "${HOME}/.local/share/ratels_pgdata" -l "${HOME}/.local/share/ratels_pgdata/server.log" -o "-p 5432 -k /tmp" start
        sleep 1
    fi
fi

# 2. Setup isolated ratels_browser_test database
echo "-> Ensuring isolated database ratels_browser_test is migrated..."
createdb -h 127.0.0.1 -p 5432 -U postgres ratels_browser_test 2>/dev/null || true
goose -dir "${ROOT_DIR}/apps/api/migrations" postgres "${TEST_DB}" up > /dev/null

# 3. Bootstrap Super Admin in ratels_browser_test
echo "-> Bootstrapping Super Admin in ratels_browser_test..."
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="${TEST_DB}" go run ./cmd/admin --super-admin --email "browser_sa@ratels.example.org" --password "BrowserAdminPass123!" --name "Chief VeryDarkMan (Super Admin)" > /dev/null 2>&1 || true)

# 4. Seed baseline geography
echo "-> Seeding baseline geography..."
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="${TEST_DB}" go run ./cmd/seed > /dev/null 2>&1 || true)
psql "${TEST_DB}" -c "UPDATE users SET status = 'active', is_verified = true WHERE email = 'member_cp@ratels.example.org';" > /dev/null 2>&1 || true

# 5. Compile server binary
echo "-> Compiling API server binary..."
SERVER_BIN="/tmp/ratels_projects_server_bin"
(cd "${ROOT_DIR}/apps/api" && go build -o "${SERVER_BIN}" ./cmd/server)

# 6. Launch API Server in background
echo "-> Starting Go API server on port ${PORT} against ratels_browser_test..."
DATABASE_URL="${TEST_DB}" \
PORT="${PORT}" \
ENVIRONMENT="development" \
APP_ENV="development" \
DEV_SAFE_DELIVERY="true" \
EMAIL_PROVIDER="dev" \
PAYMENT_PROVIDER="sandbox" \
FRONTEND_BASE_URL="http://127.0.0.1:${WEB_PORT}" \
"${SERVER_BIN}" &
API_PID=$!

cleanup() {
    echo "-> Tearing down browser test servers (PID ${API_PID:-}, ${WEB_PID:-})..."
    if [ -n "${API_PID:-}" ]; then
        kill -TERM "${API_PID}" 2>/dev/null || true
    fi
    if [ -n "${WEB_PID:-}" ]; then
        kill -TERM "${WEB_PID}" 2>/dev/null || true
    fi
    rm -f "${SERVER_BIN}"
}
trap cleanup EXIT INT TERM

# Wait for API to become ready
echo "-> Waiting for API server health endpoint..."
for i in {1..30}; do
    if curl -s "http://127.0.0.1:${PORT}/api/v1/health" | grep -q '"status":"ok"'; then
        echo "   API server ready on port ${PORT}"
        break
    fi
    sleep 0.5
done

# 7. Start Next.js frontend
echo "-> Starting Next.js frontend on port ${WEB_PORT}..."
(cd "${ROOT_DIR}/apps/web" && API_URL="http://127.0.0.1:${PORT}" PORT="${WEB_PORT}" npm run start) &
WEB_PID=$!

for i in {1..40}; do
    if curl -s -I "http://127.0.0.1:${WEB_PORT}/" 2>/dev/null | grep -q "200 OK"; then
        echo "   Web frontend ready on port ${WEB_PORT}"
        break
    fi
    sleep 0.5
done

# 8. Execute Google Chrome Headless Acceptance Suite
echo "-> Executing Google Chrome Headless Acceptance Suite for Community Projects..."
node "${ROOT_DIR}/scripts/browser_community_projects_verification.js"

echo "=========================================================="
echo " Community Projects Acceptance Suite PASSED"
echo "=========================================================="
