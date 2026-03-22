# PicoClaw Railway SSH

This repo keeps the Railway surface area small: it builds `picoclaw`, runs an OpenSSH server, and stores all PicoClaw state on the Railway volume so you can SSH in and configure everything directly.

## What you get

- Direct SSH access into the container
- PicoClaw built from source with `golang:1.25.8-alpine`
- Persistent state in `/data/.picoclaw`
- Direct access to run `picoclaw onboard`, OAuth setup, config edits, and gateway commands yourself

## How it works

- The image builds the `picoclaw` binary in a builder stage
- Runtime starts `sshd` on a TCP port
- You log in over SSH and land in a shell at `/data/.picoclaw/workspace`
- From there you can run PicoClaw commands directly

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_USERNAME` | `admin` | SSH login username |
| `SSH_PASSWORD` | *(auto-generated if no key is set)* | SSH login password |
| `SSH_PUBLIC_KEY` | empty | Public key to add to `/data/.ssh/authorized_keys` |
| `SSH_PORT` | `2222` | SSH port; falls back to `PORT` if Railway injects one |
| `PICOCLAW_VERSION` | `main` | Git branch or tag to build from upstream PicoClaw |
| `PORT` | unset | Optional Railway-provided fallback port |

## First-time setup

1. Deploy to Railway with a persistent volume mounted at `/data`
2. Expose a TCP service in Railway for the SSH port
3. Connect with `ssh <SSH_USERNAME>@<host> -p <port>`
4. Run `picoclaw onboard`
5. Complete any provider auth you need directly in the SSH session

## Useful commands

```bash
picoclaw onboard
picoclaw --help
picoclaw gateway
```

## Local testing

```bash
docker build -t picoclaw-railway-ssh .

docker run --rm -p 2222:2222 \
  -e SSH_PORT=2222 \
  -e SSH_USERNAME=admin \
  -e SSH_PASSWORD=test \
  -v $(pwd)/.tmpdata:/data \
  picoclaw-railway-ssh
```

Then connect with `ssh admin@localhost -p 2222` and use password `test`.

## Notes

- If you set `SSH_PUBLIC_KEY`, key auth is enabled using `/data/.ssh/authorized_keys`.
- If you do not set either `SSH_PASSWORD` or `SSH_PUBLIC_KEY`, startup prints `Generated SSH password: ...` in the logs.
- Password SSH is supported because you asked for it, but key auth is safer for any public deployment.
