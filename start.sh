#!/bin/sh
set -e

SSH_USERNAME="${SSH_USERNAME:-admin}"
SSH_PASSWORD="${SSH_PASSWORD:-}"
SSH_PUBLIC_KEY="${SSH_PUBLIC_KEY:-}"
SSH_PORT="${SSH_PORT:-${PORT:-2222}}"

mkdir -p /data/.picoclaw/workspace
mkdir -p /data/.picoclaw/sessions
mkdir -p /data/.picoclaw/cron
mkdir -p /var/run/sshd

if ! id "$SSH_USERNAME" >/dev/null 2>&1; then
    useradd -m -d /data -s /app/ssh-shell.sh "$SSH_USERNAME"
fi

chown -R "$SSH_USERNAME:$SSH_USERNAME" /data

if [ -z "$SSH_PASSWORD" ] && [ -z "$SSH_PUBLIC_KEY" ]; then
    SSH_PASSWORD=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 24)
    echo "Generated SSH password: $SSH_PASSWORD"
fi

if [ -n "$SSH_PASSWORD" ]; then
    echo "$SSH_USERNAME:$SSH_PASSWORD" | chpasswd
fi

install -d -m 700 -o "$SSH_USERNAME" -g "$SSH_USERNAME" /data/.ssh
AUTHORIZED_KEYS=/data/.ssh/authorized_keys
if [ -n "$SSH_PUBLIC_KEY" ]; then
    printf '%s\n' "$SSH_PUBLIC_KEY" > "$AUTHORIZED_KEYS"
    chown "$SSH_USERNAME:$SSH_USERNAME" "$AUTHORIZED_KEYS"
    chmod 600 "$AUTHORIZED_KEYS"
fi

ssh-keygen -A

cat > /etc/ssh/sshd_config <<EOF
Port $SSH_PORT
ListenAddress 0.0.0.0
Protocol 2
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_ecdsa_key
HostKey /etc/ssh/ssh_host_ed25519_key
PermitRootLogin no
PasswordAuthentication yes
KbdInteractiveAuthentication no
ChallengeResponseAuthentication no
UsePAM no
PubkeyAuthentication yes
AuthorizedKeysFile /data/.ssh/authorized_keys
AllowUsers $SSH_USERNAME
PermitUserEnvironment no
AllowTcpForwarding no
X11Forwarding no
PrintMotd no
Subsystem sftp /usr/lib/ssh/sftp-server
EOF

echo "Starting sshd on port $SSH_PORT"
echo "SSH username: $SSH_USERNAME"
if [ -n "$SSH_PUBLIC_KEY" ]; then
    echo "SSH public key auth: enabled"
fi

exec /usr/sbin/sshd -D -e -f /etc/ssh/sshd_config
