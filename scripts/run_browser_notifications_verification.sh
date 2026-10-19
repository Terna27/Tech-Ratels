#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================================="
echo " Launching Notifications & Communications Browser Suite"
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

# 3. Seed baseline geography, Super Admin, and test member
echo "-> Seeding baseline geography & Super Admin in ratels_browser_test..."
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="${TEST_DB}" go run ./cmd/seed > /dev/null 2>&1 || true)
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="${TEST_DB}" go run ./cmd/admin --super-admin --email "browser_sa@ratels.example.org" --password "BrowserAdminPass123!" --name "Chief VeryDarkMan (Super Admin)" > /dev/null 2>&1 || true)
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="${TEST_DB}" go run ./cmd/admin --email "browser_notif_member@ratels.example.org" --password "MemberSecurePass2026!" --name "Comrade Browser Ratel" --phone "+2348099887711" > /dev/null 2>&1 || true)

# 4. Compile server binary
echo "-> Compiling API server binary..."
SERVER_BIN="/tmp/ratels_notifications_server_bin"
(cd "${ROOT_DIR}/apps/api" && go build -o "${SERVER_BIN}" ./cmd/server)

# 5. Launch API Server in background
echo "-> Starting Go API server on port ${PORT} against ratels_browser_test..."
DATABASE_URL="${TEST_DB}" \
PORT="${PORT}" \
ENVIRONMENT="development" \
APP_ENV="development" \
DEV_SAFE_DELIVERY="true" \
EMAIL_PROVIDER="dev" \
ENABLE_OUTBOX_WORKER="true" \
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

# 6. Build Next.js frontend production bundle if needed
if [ ! -d "${ROOT_DIR}/apps/web/.next" ]; then
    echo "-> Building Next.js frontend production bundle..."
    (cd "${ROOT_DIR}/apps/web" && npm run build)
fi

# 7. Start Next.js frontend
echo "-> Starting Next.js frontend on port ${WEB_PORT}..."
(cd "${ROOT_DIR}/apps/web" && NEXT_PUBLIC_API_URL="http://127.0.0.1:${PORT}/api/v1" PORT="${WEB_PORT}" npm run start) &
WEB_PID=$!

for i in {1..40}; do
    if curl -s -I "http://127.0.0.1:${WEB_PORT}/" 2>/dev/null | grep -q "200 OK"; then
        echo "   Web frontend ready on port ${WEB_PORT}"
        break
    fi
    sleep 0.5
done

# 8. Run Headless Chrome Puppeteer Acceptance Suite
echo "-> Executing Google Chrome Headless Acceptance Suite for Notifications..."
chmod +x "${ROOT_DIR}/scripts/browser_notifications_verification.js"
node "${ROOT_DIR}/scripts/browser_notifications_verification.js"

echo "=========================================================="
echo " Phase B Browser Acceptance Suite Completed Successfully!"
echo "=========================================================="
