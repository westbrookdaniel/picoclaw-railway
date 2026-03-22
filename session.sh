#!/bin/bash
set -e

export HOME=/data
export PICOCLAW_HOME=/data/.picoclaw
export PICOCLAW_AGENTS_DEFAULTS_WORKSPACE=/data/.picoclaw/workspace

mkdir -p /data/.picoclaw/workspace
mkdir -p /data/.picoclaw/sessions
mkdir -p /data/.picoclaw/cron

cd /data/.picoclaw/workspace

clear
cat <<'EOF'
PicoClaw Railway Terminal

- Your persistent data lives in /data/.picoclaw
- Your default workspace is /data/.picoclaw/workspace
- Run `picoclaw onboard` if this is a fresh setup
- Run `picoclaw --help` to see available commands
EOF

exec /bin/bash -l
