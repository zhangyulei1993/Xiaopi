#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
COMPOSE="$ROOT/tests/docker-compose.test.yml"
ENV_FILE="$ROOT/tests/.env.test"
PROJECT=xiaopi-miniapp-test
ACTION=${1:-status}

compose() {
  docker compose --project-name "$PROJECT" --file "$COMPOSE" --env-file "$ENV_FILE" "$@"
}

require_docker() {
  docker info >/dev/null 2>&1 || { echo "Docker daemon is not available" >&2; exit 1; }
}

make_env() {
  if [ -e "$ENV_FILE" ]; then
    echo "Refusing to overwrite existing $ENV_FILE" >&2
    exit 1
  fi
  umask 077
  cat > "$ENV_FILE" <<EOF
TEST_MYSQL_USER=qa_runner
TEST_MYSQL_PASSWORD=$(openssl rand -hex 24)
TEST_MYSQL_ROOT_PASSWORD=$(openssl rand -hex 32)
TEST_AUTH_TOKEN_SECRET=$(openssl rand -hex 32)
TEST_AUTH_TOKEN_TTL=15m
TEST_BOOTSTRAP_ADMIN_PASSWORD=$(openssl rand -hex 24)
TEST_WECHAT_CODE_MAP=qa-user-a=qa-user-a,qa-user-b=qa-user-b,qa-admin=qa-admin,qa-disabled=qa-disabled
TEST_ACCESS_CODE_MAP=13800000001:100001=qa-user-a,13800000002:100002=qa-user-b,13800000003:100003=qa-admin,13800000004:100004=qa-disabled
EOF
  chmod 600 "$ENV_FILE"
}

verify_compose() {
  compose config --format json | node "$ROOT/tests/scripts/verify-compose.mjs" --allow-internal
  for volume in xiaopi_miniapp_e2e_test_mysql_data; do
    if docker volume inspect "$volume" >/dev/null 2>&1; then
      owner=$(docker volume inspect -f '{{ index .Labels "com.docker.compose.project" }}' "$volume")
      [ "$owner" = "$PROJECT" ] || { echo "Refusing to use volume $volume owned by ${owner:-another project}" >&2; exit 1; }
    fi
  done
  for network in xiaopi_miniapp_e2e_test_internal; do
    if docker network inspect "$network" >/dev/null 2>&1; then
      owner=$(docker network inspect -f '{{ index .Labels "com.docker.compose.project" }}' "$network")
      [ "$owner" = "$PROJECT" ] || { echo "Refusing to use network $network owned by ${owner:-another project}" >&2; exit 1; }
    fi
  done
}

wait_api() {
  attempt=0
  until curl --fail --silent --max-time 2 http://127.0.0.1:18083/api/health >/dev/null; do
    attempt=$((attempt + 1))
    [ "$attempt" -lt 30 ] || { echo "FAIL: test API health check timed out after 60 seconds" >&2; compose logs --tail=80 api-test >&2; return 1; }
    sleep 2
  done
}

check_mysql() {
  actual_db=$(compose exec -T mysql-test sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysqladmin ping -h127.0.0.1 -u"$MYSQL_USER" --connect-timeout=3' 2>&1) || { echo "FAIL: test MySQL health check failed: $actual_db" >&2; return 1; }
  actual_db=$(compose exec -T mysql-test sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" --connect-timeout=3 -Nse "SELECT DATABASE()" "$MYSQL_DATABASE"' 2>&1) || { echo "FAIL: test MySQL query failed: $actual_db" >&2; return 1; }
  [ "$actual_db" = xiaopi_miniapp_test ] || { echo "FAIL: unexpected MySQL database: $actual_db" >&2; return 1; }
  echo "PASS: test MySQL reachable; selected database is xiaopi_miniapp_test."
}

check_api() {
  curl --fail --silent --show-error --max-time 3 http://127.0.0.1:18083/api/health >/dev/null || { echo "FAIL: test API health endpoint failed at 127.0.0.1:18083" >&2; return 1; }
  echo "PASS: test API health endpoint responds on loopback."
}

seed() {
  actual_db=$(compose exec -T mysql-test sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -Nse "SELECT DATABASE()" "$MYSQL_DATABASE"')
  [ "$actual_db" = xiaopi_miniapp_test ] || { echo "Refusing seed: DB target is $actual_db" >&2; exit 1; }
  compose exec -T mysql-test sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" "$MYSQL_DATABASE"' < "$ROOT/tests/seed.test.sql"
}

case "$ACTION" in
  config)
    [ -f "$ENV_FILE" ] || make_env
    require_docker
    verify_compose
    echo "Test-only credentials created at tests/.env.test (mode 600); no services were started."
    ;;
  init)
    require_docker
    [ -f "$ENV_FILE" ] || make_env
    verify_compose
    compose up -d --build mysql-test api-test
    wait_api
    seed
    echo "Isolated test API ready at http://127.0.0.1:18083/api; seeded synthetic QA fixtures."
    ;;
  status)
    require_docker
    [ -f "$ENV_FILE" ] || { echo "Test environment not initialized"; exit 2; }
    verify_compose
    compose ps
    check_mysql
    check_api
    ;;
  reset)
    require_docker
    [ -f "$ENV_FILE" ] || { echo "Test environment not initialized" >&2; exit 2; }
    verify_compose
    wait_api
    actual_db=$(compose exec -T mysql-test sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -Nse "SELECT DATABASE()" "$MYSQL_DATABASE"')
    [ "$actual_db" = xiaopi_miniapp_test ] || { echo "Refusing reset: DB target is $actual_db" >&2; exit 1; }
    compose exec -T mysql-test sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" "$MYSQL_DATABASE"' < "$ROOT/tests/reset.test.sql"
    seed
    echo "Synthetic QA records reset in $actual_db."
    ;;
  down)
    require_docker
    [ -f "$ENV_FILE" ] || { echo "Test environment not initialized" >&2; exit 2; }
    verify_compose
    compose down --remove-orphans
    echo "Isolated test services stopped; the test-only volume was preserved."
    ;;
  *)
    echo "Usage: $0 {init|status|reset|down}" >&2
    exit 2
    ;;
esac
