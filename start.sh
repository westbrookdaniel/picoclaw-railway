#!/bin/sh
set -e

mkdir -p /data/.picoclaw/workspace
mkdir -p /data/.picoclaw/sessions
mkdir -p /data/.picoclaw/cron

TTYD_USERNAME="${TTYD_USERNAME:-admin}"
TTYD_PASSWORD="${TTYD_PASSWORD:-}"
PORT="${PORT:-8080}"

if [ -z "$TTYD_PASSWORD" ]; then
    TTYD_PASSWORD=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24)
    echo "Generated terminal password: $TTYD_PASSWORD"
fi

echo "Starting ttyd on port $PORT"
echo "Terminal username: $TTYD_USERNAME"

exec ttyd \
    -W \
    -m 1 \
    -O \
    -p "$PORT" \
    -c "$TTYD_USERNAME:$TTYD_PASSWORD" \
    /app/session.sh
