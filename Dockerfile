FROM golang:1.25.8-alpine AS picoclaw-builder

RUN apk add --no-cache git make

WORKDIR /src

ARG PICOCLAW_VERSION=main

RUN git clone --depth 1 --branch ${PICOCLAW_VERSION} https://github.com/sipeed/picoclaw.git .
RUN go mod download
RUN make build

FROM alpine:3.22

RUN apk add --no-cache bash ca-certificates curl git jq less openssh procps shadow

COPY --from=picoclaw-builder /src/build/picoclaw /usr/local/bin/picoclaw

RUN mkdir -p /app /data/.picoclaw

COPY start.sh /app/start.sh
COPY ssh-shell.sh /app/ssh-shell.sh
RUN chmod +x /app/start.sh
RUN chmod +x /app/ssh-shell.sh

ENV HOME=/data
ENV PICOCLAW_HOME=/data/.picoclaw
ENV PICOCLAW_AGENTS_DEFAULTS_WORKSPACE=/data/.picoclaw/workspace

CMD ["/app/start.sh"]
