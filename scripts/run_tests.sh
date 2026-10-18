#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=========================================================="
echo " Running Nigerian Ratels Phase 1 Automated Tests (Go)"
echo "=========================================================="

export PATH="${HOME}/go/bin:${HOME}/.local/bin:${PATH}"
export GOTOOLCHAIN=auto

# Ensure PostgreSQL is active
if ! pg_isready -h 127.0.0.1 -p 5432 -q; then
    if [ -d "${HOME}/.local/share/ratels_pgdata" ]; then
        pg_ctl -D "${HOME}/.local/share/ratels_pgdata" -l "${HOME}/.local/share/ratels_pgdata/server.log" -o "-p 5432 -k /tmp" start
        sleep 1
    fi
fi

# Ensure test database exists and is migrated cleanly
dropdb -h 127.0.0.1 -p 5432 -U postgres ratels_test 2>/dev/null || true
createdb -h 127.0.0.1 -p 5432 -U postgres ratels_test
goose -dir "${ROOT_DIR}/apps/api/migrations" postgres "postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable" up

# Run seeds on test DB
(cd "${ROOT_DIR}/apps/api" && DATABASE_URL="postgres://postgres@127.0.0.1:5432/ratels_test?sslmode=disable" go run ./cmd/seed)

# Execute all Go test suites
cd "${ROOT_DIR}/apps/api"
go test -v ./...

echo "=========================================================="
echo " All Phase 1 Go Automated Tests Passed Successfully"
echo "=========================================================="
