#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================================="
echo " Starting Nigerian Ratels Management System (Go Backend)"
echo "=========================================================="

export PATH="${HOME}/go/bin:${HOME}/.local/bin:${PATH}"
export GOTOOLCHAIN=auto

# 1. Ensure PostgreSQL is active
if ! pg_isready -h 127.0.0.1 -p 5432 -q; then
    echo "-> Starting local PostgreSQL instance..."
    if [ -d "${HOME}/.local/share/ratels_pgdata" ]; then
        pg_ctl -D "${HOME}/.local/share/ratels_pgdata" -l "${HOME}/.local/share/ratels_pgdata/server.log" -o "-p 5432 -k /tmp" start
        sleep 1
    fi
fi

# Ensure databases exist
createdb -h 127.0.0.1 -p 5432 -U postgres ratels_dev 2>/dev/null || true
createdb -h 127.0.0.1 -p 5432 -U postgres ratels_test 2>/dev/null || true

# 2. Run goose migrations
echo "-> Applying database migrations via goose..."
goose -dir "${ROOT_DIR}/apps/api/migrations" postgres "postgres://postgres@127.0.0.1:5432/ratels_dev?sslmode=disable" up

# 3. Run idempotent seed
echo "-> Populating reference geography, institutional data, and badges..."
cd "${ROOT_DIR}/apps/api"
SEED_FLAG=""
if [ "${SEED_DEMO_ACCOUNTS:-}" = "true" ]; then
    SEED_FLAG="--demo-accounts"
fi
go run ./cmd/seed ${SEED_FLAG} "$@"

# 4. Start Go Backend
echo "-> Starting Go API Backend on http://127.0.0.1:8000..."
go run ./cmd/server &
API_PID=$!

# 5. Start Next.js Frontend
echo "-> Starting Next.js Web Frontend on http://localhost:3000..."
cd "${ROOT_DIR}/apps/web"
npm run dev &
WEB_PID=$!

trap "kill ${API_PID} ${WEB_PID} 2>/dev/null || true; exit" INT TERM EXIT

wait
