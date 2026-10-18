#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================================="
echo " Launching Marketplace Browser Acceptance Suite"
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

# 4. Seed reference geography and marketplace merchandise
echo "-> Seeding baseline marketplace merchandise and reference data..."
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="${TEST_DB}" go run ./cmd/seed > /dev/null)

# 5. Compile server binary
echo "-> Compiling API server binary..."
(cd "${ROOT_DIR}/apps/api" && go build -o /tmp/ratels-browser-server ./cmd/server)

# 6. Start API Server on port 8000 pointing to ratels_browser_test
echo "-> Starting API server on port ${PORT} against ratels_browser_test..."
PORT=${PORT} DATABASE_URL="${TEST_DB}" /tmp/ratels-browser-server &
API_PID=$!

# Wait for API server ready
for i in {1..30}; do
    if curl -s "http://127.0.0.1:${PORT}/api/v1/health" | grep -q '"status":"ok"'; then
        echo "   API server ready on port ${PORT}"
        break
    fi
    sleep 0.5
done

# 7. Start Next.js frontend on port 3000
echo "-> Building Next.js frontend production bundle..."
(cd "${ROOT_DIR}/apps/web" && npm run build > /dev/null)
echo "-> Starting Next.js frontend on port ${WEB_PORT} with API reverse proxy..."
(cd "${ROOT_DIR}/apps/web" && API_URL="http://127.0.0.1:${PORT}" PORT=${WEB_PORT} npm run start) &
WEB_PID=$!

# Clean cleanup trap
cleanup() {
    echo "-> Tearing down browser test servers (PID ${API_PID}, ${WEB_PID})..."
    kill "${API_PID}" "${WEB_PID}" 2>/dev/null || true
    wait "${API_PID}" "${WEB_PID}" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

# Wait for Web frontend ready
for i in {1..40}; do
    if curl -s -I "http://localhost:${WEB_PORT}" | grep -q "200 OK"; then
        echo "   Web frontend ready on port ${WEB_PORT}"
        break
    fi
    sleep 0.5
done

# 8. Execute Browser Verification Suite
echo "-> Executing Google Chrome Headless Acceptance Suite for Marketplace..."
node "${ROOT_DIR}/scripts/browser_marketplace_verification.js"

echo "=========================================================="
echo " Marketplace Browser Acceptance Verification PASSED"
echo "=========================================================="
