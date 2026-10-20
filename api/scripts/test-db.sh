#!/usr/bin/env bash
# Roda os testes da API com um PostgreSQL descartável, como o CI faz.
# Uso: bash scripts/test-db.sh [argumentos do go test]   (padrão: ./...)
# O container é removido ao final; nenhum banco real é tocado.
set -euo pipefail

readonly CONTAINER="easy-fut-test-db-$$"
readonly PORT="${TEST_DB_PORT:-55433}"

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run --rm -d --name "$CONTAINER" -p "127.0.0.1:$PORT:5432" \
  -e POSTGRES_USER=futapp -e POSTGRES_PASSWORD=test-only -e POSTGRES_DB=futapp_test \
  postgres:17-alpine >/dev/null

for _ in $(seq 1 30); do
  docker exec "$CONTAINER" pg_isready -U futapp -d futapp_test >/dev/null 2>&1 && break
  sleep 1
done

cd "$(dirname "$0")/.."
TEST_DATABASE_URL="postgres://futapp:test-only@127.0.0.1:$PORT/futapp_test?sslmode=disable" \
  go test -count=1 "${@:-./...}"
