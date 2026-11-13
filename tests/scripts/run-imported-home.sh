#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
ENV_FILE="$ROOT/tests/.env.test"
COMPOSE="$ROOT/tests/docker-compose.test.yml"
DB_CONTAINER=xiaopi-miniapp-test-mysql-test-1
API_CONTAINER=xiaopi-miniapp-test-api-test-1
DATABASE=xiaopi_miniapp_test
IMPORT=/tmp/xiaopi-platform-local-test.sql
BACKUP=$(mktemp /tmp/xiaopi-test-db-before-import.XXXXXX.sql)
IMPORT_LOCAL=$(mktemp /tmp/xiaopi-imported-home.XXXXXX.sql)
RESTORE_NEEDED=0

restore() {
  result=$?
  trap - EXIT INT TERM
  if [ "$RESTORE_NEEDED" = 1 ]; then
    if docker exec "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -Nse "SELECT DATABASE()" "$MYSQL_DATABASE"' | grep -qx "$DATABASE"; then
      docker exec -i "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" "$MYSQL_DATABASE"' < "$BACKUP" || result=1
      echo "Restored isolated QA database tables from pre-run snapshot."
    else
      echo "ERROR: database target changed; retained backup at $BACKUP" >&2
      result=1
    fi
  fi
  rm -f "$BACKUP"
  rm -f "$IMPORT_LOCAL"
  exit "$result"
}
trap restore EXIT INT TERM

[ -f "$ENV_FILE" ] || { echo 'Test-only environment is not initialized.' >&2; exit 2; }
[ -f "$IMPORT" ] || { echo "Sanitized import is missing: $IMPORT" >&2; exit 2; }
docker compose --project-name xiaopi-miniapp-test --file "$COMPOSE" --env-file "$ENV_FILE" config --format json | node "$ROOT/tests/scripts/verify-compose.mjs" --allow-internal
docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$DB_CONTAINER" | grep -qx xiaopi-miniapp-test
docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$API_CONTAINER" | grep -qx xiaopi-miniapp-test
docker exec "$API_CONTAINER" sh -ec '[ "$APP_ENV" = test ] && [ "$MYSQL_HOST" = mysql-test ] && [ "$MYSQL_DATABASE" = xiaopi_miniapp_test ]'
docker exec "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysqldump -h127.0.0.1 -u"$MYSQL_USER" --single-transaction --skip-lock-tables --no-tablespaces --skip-add-locks "$MYSQL_DATABASE" properties property_media building_rules app_configs' > "$BACKUP"
RESTORE_NEEDED=1
docker exec "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -Nse "SELECT DATABASE()" "$MYSQL_DATABASE"' | grep -qx "$DATABASE"
sed 's#https://cos\.xiaopizufang\.cn/media/[^'"'"']*#http://127.0.0.1:18083/uploads/qa-ui-visual.png#g' "$IMPORT" > "$IMPORT_LOCAL"
if rg -n 'https?://(?!127\.0\.0\.1)' "$IMPORT_LOCAL" --pcre2; then
  echo 'Refusing to import a non-loopback URL into the egress-isolated test network.' >&2
  exit 1
fi
docker exec -i "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" "$MYSQL_DATABASE"' < "$IMPORT_LOCAL"
docker exec "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" "$MYSQL_DATABASE" -e "INSERT INTO properties (id,title,community,location,layout,rent_monthly,fee_text,description,status,owner_id) VALUES (\"qa-property-1\",\"TEST Listing A\",\"TEST Community A\",\"TEST Address\",\"1bd 1ba\",1800,\"TEST - no broker fee\",\"Synthetic test fixture only\",\"available\",\"qa-user-a\") ON DUPLICATE KEY UPDATE title=VALUES(title),community=VALUES(community),location=VALUES(location),layout=VALUES(layout),rent_monthly=VALUES(rent_monthly),fee_text=VALUES(fee_text),description=VALUES(description),status=\"available\",owner_id=\"qa-user-a\""'
docker exec "$DB_CONTAINER" sh -ec 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -h127.0.0.1 -u"$MYSQL_USER" -N "$MYSQL_DATABASE" -e "SELECT COUNT(*) FROM properties WHERE id!=\"qa-property-1\" AND status=\"available\"; SELECT COALESCE(JSON_UNQUOTE(JSON_EXTRACT(config_value,\"$.contactPhone\")),\"\") FROM app_configs WHERE config_key=\"miniapp_public\""' | awk 'NR==1 && $0==4 {next} NR==2 && $0=="" {next} {exit 1} END {if(NR!=2) exit 1}'
echo 'Imported four sanitized production listings into isolated QA database; running the scoped home load and visual checks.'
(cd "$ROOT/miniapp" && npm run test -- --cases=home-load --visual=home --listing-data=imported)
