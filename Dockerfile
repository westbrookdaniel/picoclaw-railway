FROM golang:1.25.8-alpine AS picoclaw-builder

RUN apk add --no-cache git make

WORKDIR /src

ARG PICOCLAW_VERSION=main

RUN git clone --depth 1 --branch ${PICOCLAW_VERSION} https://github.com/sipeed/picoclaw.git .
RUN go mod download
RUN make build

FROM alpine:3.22

ARG TTYD_VERSION=1.7.7

RUN apk add --no-cache bash ca-certificates curl git jq less procps && \
    arch="$(apk --print-arch)" && \
    case "$arch" in \
        x86_64) ttyd_arch="x86_64" ;; \
        aarch64) ttyd_arch="aarch64" ;; \
        *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${ttyd_arch}" -o /usr/local/bin/ttyd && \
    chmod +x /usr/local/bin/ttyd

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
