#!/usr/bin/env bash
# Rebuild only the API service on the production host.
# The real production secrets intentionally live at /opt/xiaopi/.env, not in
# docker/.env. Keep the path explicit so Compose never starts with empty vars.
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_DIR="$REPO_ROOT/docker"
ENV_FILE="${XIAOPI_ENV_FILE:-$REPO_ROOT/.env}"

if [[ ! -r "$ENV_FILE" ]]; then
  echo "Production environment file is not readable: $ENV_FILE" >&2
  exit 1
fi

cd "$COMPOSE_DIR"
docker compose --env-file "$ENV_FILE" \
  -f docker-compose.yml \
  -f docker-compose.server.yml \
  up -d --build --no-deps api
docker compose --env-file "$ENV_FILE" \
  -f docker-compose.yml \
  -f docker-compose.server.yml \
  ps api
