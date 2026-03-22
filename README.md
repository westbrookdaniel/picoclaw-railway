# PicoClaw Railway Terminal

This repo keeps the Railway surface area small: it builds `picoclaw`, exposes a password-protected browser terminal with GoTTY, and stores all PicoClaw state on the Railway volume.

## What you get

- A single browser terminal instead of a custom admin UI
- PicoClaw built from source with `golang:1.25.8-alpine`
- Persistent state in `/data/.picoclaw`
- Direct access to run `picoclaw onboard`, OAuth setup, config edits, and gateway commands yourself

## How it works

- The image builds the `picoclaw` binary in a builder stage
- Runtime starts GoTTY on Railway's HTTP port
- You log in with Basic Auth, then land in a shell at `/data/.picoclaw/workspace`
- From there you can run PicoClaw commands directly

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TERMINAL_USERNAME` | `admin` | Browser terminal username |
| `TERMINAL_PASSWORD` | *(auto-generated)* | Browser terminal password; if omitted, it is printed in deploy logs |
| `TTYD_USERNAME` | `admin` | Backward-compatible alias for `TERMINAL_USERNAME` |
| `TTYD_PASSWORD` | *(auto-generated)* | Backward-compatible alias for `TERMINAL_PASSWORD` |
| `PICOCLAW_VERSION` | `main` | Git branch or tag to build from upstream PicoClaw |
| `PORT` | `8080` | HTTP port used by Railway |

## First-time setup

1. Deploy to Railway with a persistent volume mounted at `/data`
2. Add a public domain
3. Open the domain and log in with `TERMINAL_USERNAME` / `TERMINAL_PASSWORD`
4. Run `picoclaw onboard`
5. Complete any provider auth you need directly in the terminal

## Useful commands

```bash
picoclaw onboard
picoclaw --help
picoclaw gateway
```

## Local testing

```bash
docker build -t picoclaw-railway-terminal .

docker run --rm -p 8080:8080 \
  -e PORT=8080 \
  -e TERMINAL_USERNAME=admin \
  -e TERMINAL_PASSWORD=test \
  -v $(pwd)/.tmpdata:/data \
  picoclaw-railway-terminal
```

Then open `http://localhost:8080` and log in with `admin` / `test`.

## Notes

- If you do not set `TERMINAL_PASSWORD`, startup prints `Generated terminal password: ...` in the logs.
- This now uses GoTTY instead of ttyd because ttyd's websocket flow was being rejected behind Railway's proxy.
- Railway health checks are disabled here because the terminal is auth-protected at `/`.
