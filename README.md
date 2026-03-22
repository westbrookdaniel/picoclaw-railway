# PicoClaw Railway SSH

This repo builds `picoclaw`, runs the PicoClaw gateway automatically, exposes the gateway through Caddy with Basic Auth on Railway HTTP, and keeps OpenSSH available for direct admin access over Railway TCP.

## What you get

- Direct SSH access into the container
- Always-on PicoClaw gateway inside the container
- Public HTTP access to the gateway behind Basic Auth
- PicoClaw built from source with `golang:1.25.8-alpine`
- Persistent state in `/data/.picoclaw`
- Direct access to run `picoclaw onboard`, OAuth setup, config edits, and gateway commands yourself

## How it works

- The image builds the `picoclaw` binary in a builder stage
- Runtime starts `sshd` on a TCP port
- Runtime starts `picoclaw gateway` on `127.0.0.1:18790`
- Runtime starts Caddy on `0.0.0.0:8080` and proxies to the local gateway
- You log in over SSH and land in a shell at `/data/.picoclaw/workspace`
- From there you can run PicoClaw commands directly

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_USERNAME` | `admin` | SSH login username |
| `SSH_PASSWORD` | *(auto-generated if no key is set)* | SSH login password |
| `SSH_PUBLIC_KEY` | empty | Public key to add to `/data/.ssh/authorized_keys` |
| `SSH_PORT` | `2222` | SSH port used inside the container |
| `GATEWAY_PUBLIC_PORT` | `8080` | Public HTTP port used by Caddy |
| `GATEWAY_BASIC_AUTH_USERNAME` | `admin` | Username protecting the public PicoClaw gateway |
| `GATEWAY_BASIC_AUTH_PASSWORD` | *(auto-generated)* | Password protecting the public PicoClaw gateway |
| `PICOCLAW_GATEWAY_HOST` | `127.0.0.1` | Local bind target Caddy proxies to |
| `PICOCLAW_GATEWAY_PORT` | `18790` | Local PicoClaw gateway port |
| `PICOCLAW_VERSION` | `main` | Git branch or tag to build from upstream PicoClaw |

## First-time setup

1. Deploy to Railway with a persistent volume mounted at `/data`
2. Add a Railway public HTTP domain pointing at internal port `8080`
3. Expose a Railway TCP service pointing at internal port `2222`
4. Open the public HTTP domain and log in with `GATEWAY_BASIC_AUTH_USERNAME` / `GATEWAY_BASIC_AUTH_PASSWORD`
5. Connect with `ssh <SSH_USERNAME>@<host> -p <port>` for admin access
6. Run `picoclaw onboard` over SSH to finish provider setup

## Railway ports

- Public HTTP domain -> internal port `8080`
- TCP proxy for SSH -> internal port `2222`
- PicoClaw gateway itself stays private on `127.0.0.1:18790`

## First login and Codex setup

1. SSH into the container
2. Run:

```bash
picoclaw onboard
```

3. Choose the OpenAI/Codex option in onboarding if offered
4. Complete OAuth or API-key setup from the SSH session
5. Once configured, use the public Railway URL for the gateway UI

## Useful commands

```bash
picoclaw onboard
picoclaw --help
picoclaw gateway
cat /data/.picoclaw/config.json
```

## Local testing

```bash
docker build -t picoclaw-railway-ssh .

docker run --rm -p 2222:2222 -p 8080:8080 \
  -e SSH_PORT=2222 \
  -e SSH_USERNAME=admin \
  -e SSH_PASSWORD=test \
  -e GATEWAY_PUBLIC_PORT=8080 \
  -e GATEWAY_BASIC_AUTH_USERNAME=admin \
  -e GATEWAY_BASIC_AUTH_PASSWORD=gatewaytest \
  -v $(pwd)/.tmpdata:/data \
  picoclaw-railway-ssh
```

Then:

- connect with `ssh admin@localhost -p 2222` and use password `test`
- open `http://localhost:8080` and log in with `admin` / `gatewaytest`

## Notes

- If you set `SSH_PUBLIC_KEY`, key auth is enabled using `/data/.ssh/authorized_keys`.
- If you do not set either `SSH_PASSWORD` or `SSH_PUBLIC_KEY`, startup prints `Generated SSH password: ...` in the logs.
- If you do not set `GATEWAY_BASIC_AUTH_PASSWORD`, startup prints `Generated gateway password: ...` in the logs.
- SSH host keys are stored under `/data/.ssh/host_keys` so clients do not see a new host fingerprint on every redeploy.
- Password SSH is supported because you asked for it, but key auth is safer for any public deployment.
