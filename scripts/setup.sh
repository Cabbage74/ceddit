#!/usr/bin/env bash
# setup.sh — ensure infrastructure is ready before starting the app.
# Called by `make run` automatically.

set -e

MYSQL_PORT="${MYSQL_PORT:-13306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASS="${MYSQL_PASS:-040703}"

# ---- start docker compose if not running ----
if ! docker ps --format '{{.Names}}' | grep -q 'ceddit-mysql'; then
    echo "[setup] starting docker compose..."
    docker compose up -d
else
    echo "[setup] containers already running"
fi

# ---- wait for MySQL ----
echo -n "[setup] waiting for MySQL"
for i in $(seq 1 30); do
    if docker exec ceddit-mysql mysqladmin ping -h localhost --silent 2>/dev/null; then
        echo " ready"
        break
    fi
    echo -n "."
    sleep 1
done

# ---- grant Canal replication privileges ----
echo "[setup] granting Canal replication privileges..."
docker exec ceddit-mysql mysql -u"$MYSQL_USER" -p"$MYSQL_PASS" \
    -e "GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'root'@'%'; FLUSH PRIVILEGES;" 2>/dev/null || true
echo "[setup] done"
