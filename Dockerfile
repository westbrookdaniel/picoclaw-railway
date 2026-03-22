FROM golang:1.25.8-alpine AS picoclaw-builder

RUN apk add --no-cache git make

WORKDIR /src

ARG PICOCLAW_VERSION=main

RUN git clone --depth 1 --branch ${PICOCLAW_VERSION} https://github.com/sipeed/picoclaw.git .
RUN go mod download
RUN make build

FROM alpine:3.22

ARG GOTTY_VERSION=v1.0.1

RUN apk add --no-cache bash ca-certificates curl git jq less procps tar && \
    arch="$(apk --print-arch)" && \
    case "$arch" in \
        x86_64) gotty_arch="amd64" ;; \
        armv7|armhf) gotty_arch="arm" ;; \
        *) echo "Unsupported architecture for GoTTY: $arch" >&2; exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/yudai/gotty/releases/download/${GOTTY_VERSION}/gotty_linux_${gotty_arch}.tar.gz" -o /tmp/gotty.tar.gz && \
    tar -xzf /tmp/gotty.tar.gz -C /usr/local/bin gotty && \
    chmod +x /usr/local/bin/gotty && \
    rm -f /tmp/gotty.tar.gz

COPY --from=picoclaw-builder /src/build/picoclaw /usr/local/bin/picoclaw

RUN mkdir -p /app /data/.picoclaw

COPY start.sh /app/start.sh
COPY session.sh /app/session.sh
RUN chmod +x /app/start.sh
RUN chmod +x /app/session.sh

ENV HOME=/data
ENV PICOCLAW_HOME=/data/.picoclaw
ENV PICOCLAW_AGENTS_DEFAULTS_WORKSPACE=/data/.picoclaw/workspace

CMD ["/app/start.sh"]
