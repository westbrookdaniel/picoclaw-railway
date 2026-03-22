#!/bin/sh
set -e

mkdir -p /data/.picoclaw/workspace
mkdir -p /data/.picoclaw/sessions
mkdir -p /data/.picoclaw/cron

TERMINAL_USERNAME="${TERMINAL_USERNAME:-${TTYD_USERNAME:-admin}}"
TERMINAL_PASSWORD="${TERMINAL_PASSWORD:-${TTYD_PASSWORD:-}}"
PORT="${PORT:-8080}"

if [ -z "$TERMINAL_PASSWORD" ]; then
    TERMINAL_PASSWORD=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24)
    echo "Generated terminal password: $TERMINAL_PASSWORD"
fi

echo "Starting GoTTY on port $PORT"
echo "Terminal username: $TERMINAL_USERNAME"

exec gotty \
    --permit-write \
    --reconnect \
    --reconnect-time 10 \
    -p "$PORT" \
    -c "$TERMINAL_USERNAME:$TERMINAL_PASSWORD" \
    /app/session.sh
